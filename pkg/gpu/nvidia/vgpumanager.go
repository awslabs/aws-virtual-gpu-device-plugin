package nvidia

import (
	"errors"
	"fmt"
	"syscall"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
	"github.com/fsnotify/fsnotify"
	log "github.com/golang/glog"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

type vGPUManager struct {
	enableMPS          bool
	enabvleHealthCheck bool
}

func NewVirtualGPUManager(enableMPS, enabvleHealthCheck bool, memoryUnit int) *sharedGPUManager {
	return &sharedGPUManager{
		enableMPS:          enableMPS,
		enabvleHealthCheck: enabvleHealthCheck,
		memoryUnit:         memoryUnit,
	}
}

func (vgm *vGPUManager) Run() error {
	log.Println("Loading NVML")
	if err := nvml.Init(); err != nil {
		log.Printf("Failed to initialize NVML: %s.", err)
		log.Printf("If this is a GPU node, did you set the docker default runtime to `nvidia`?")

		// TODO: point to our repo
		log.Printf("You can check the prerequisites at: https://github.com/NVIDIA/k8s-device-plugin#prerequisites")
		log.Printf("You can learn how to set the runtime at: https://github.com/NVIDIA/k8s-device-plugin#quick-start")

		select {}
	}
	defer func() { log.Println("Shutdown of NVML returned:", nvml.Shutdown()) }()

	// Check if MemoryUnit is a valid value.
	if err := validMemoryUnit(vgm.memoryUnit); err != nil {
		log.Printf("Failed to valid memoryUnit: %s.", err)
		return err
	}

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

			devicePlugin = NewNvidiaDevicePlugin(vgm.enableMPS, vgm.enableHealthCheck, vgm.memoryUnit)
			if err := devicePlugin.Serve(); err != nil {
				log.Println("Could not contact Kubelet, retrying. Did you enable the device plugin feature gate?")
				// TODO: point to our own docs.
				log.Printf("You can check the prerequisites at: https://github.com/NVIDIA/k8s-device-plugin#prerequisites")
				log.Printf("You can learn how to set the runtime at: https://github.com/NVIDIA/k8s-device-plugin#quick-start")
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

}

// retrieve one GPU and check memory / MemoryUnit is equals 0 or not.
func validMemoryUnit(memoryUnit int) error {
	n, err := nvml.GetDeviceCount()
	check(err)

	if n <= 0 {
		return errors.New("Can not find available GPU on the node")
	}

	// AWS EC2 has exact same GPUs on single instance so it's safe to pick any one for validation
	d, err := nvml.NewDevice(0)
	check(err)

	if uint(*d.Memory)%uint(memoryUnit) != 0 {
		return errors.New(fmt.Sprintf("Current GPU Model %s has total memory %d which can not divided by MemoryUnit %d", *d.Model, *d.Memory, memoryUnit))
	}

	return nil
}
