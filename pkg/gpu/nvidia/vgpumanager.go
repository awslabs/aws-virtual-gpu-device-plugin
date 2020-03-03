package nvidia

import (
	"syscall"

	"log"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
	"github.com/fsnotify/fsnotify"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

type vGPUManager struct {
	vGPUCount int
}

// NewVirtualGPUManager create a instance of vGPUManager
func NewVirtualGPUManager(vGPUCount int) *vGPUManager {
	return &vGPUManager{
		vGPUCount: vGPUCount,
	}
}

func (vgm *vGPUManager) Run() error {
	log.Println("Loading NVML")
	if err := nvml.Init(); err != nil {
		log.Printf("Failed to initialize NVML: %s.", err)
		log.Printf("If this is a GPU node, did you set the docker default runtime to `nvidia`?")

		log.Printf("You can check the prerequisites at: https://github.com/awslabs/aws-virtual-gpu-device-plugin#prerequisites")
		log.Printf("You can learn how to set the runtime at: https://github.com/awslabs/k8s-virtual-gpu#quick-start")

		select {}
	}
	defer func() { log.Println("Shutdown of NVML returned:", nvml.Shutdown()) }()

	log.Println("Fetching devices.")
	if getDeviceCount() == 0 {
		log.Println("No devices found. Waiting indefinitely.")
		select {}
	}

	log.Println("Starting FS watcher.")
	watcher, err := newFSWatcher(pluginapi.DevicePluginPath)
	if err != nil {
		log.Println("Failed to created FS watcher.")
		return err
	}
	defer watcher.Close()

	log.Println("Starting OS watcher.")
	sigs := newOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	restart := true
	var devicePlugin *NvidiaDevicePlugin

L:
	for {
		if restart {
			if devicePlugin != nil {
				devicePlugin.Stop()
			}

			devicePlugin = NewNvidiaDevicePlugin(vgm.vGPUCount)
			if err := devicePlugin.Serve(); err != nil {
				log.Printf("You can check the prerequisites at: https://github.com/awslabs/aws-virtual-gpu-device-plugin#prerequisites")
				log.Printf("You can learn how to set the runtime at: https://github.com/awslabs/aws-virtual-gpu-device-plugin#quick-start")
			} else {
				restart = false
			}
		}

		select {
		case event := <-watcher.Events:
			if event.Name == pluginapi.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				log.Printf("inotify: %s created, restarting.", pluginapi.KubeletSocket)
				restart = true
			}

		case err := <-watcher.Errors:
			log.Printf("inotify: %s", err)

		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				log.Println("Received SIGHUP, restarting.")
				restart = true
			default:
				log.Printf("Received signal \"%v\", shutting down.", s)
				devicePlugin.Stop()
				break L
			}
		}
	}

	return nil
}
