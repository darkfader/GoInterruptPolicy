package main

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	modSetupapi = windows.NewLazyDLL("setupapi.dll")

	procSetupDiGetClassDevsW = modSetupapi.NewProc("SetupDiGetClassDevsW")
)

func FindAllDevices() []Device {
	var allDevices []Device
	handle, err := SetupDiGetClassDevs(nil, nil, 0, 0xe)
	if err != nil {
		panic(err)
	}
	defer SetupDiDestroyDeviceInfoList(handle)

	var index = 0
	for {
		idata, err := SetupDiEnumDeviceInfo(handle, index)
		if err != nil {
			break
		}
		index++
		dev := Device{}

		val, err := SetupDiGetDeviceRegistryProperty(handle, idata, SPDRP_DEVICEDESC)
		if err == nil {
			if val.(string) == "" {
				continue
			}
			dev.DeviceDesc = val.(string)
		}

		val, err = SetupDiGetDeviceRegistryProperty(handle, idata, SPDRP_FRIENDLYNAME)
		if err == nil {
			dev.FriendlyName = val.(string)
		}

		val, err = SetupDiGetDeviceRegistryProperty(handle, idata, SPDRP_PHYSICAL_DEVICE_OBJECT_NAME)
		if err == nil {
			dev.DevObjName = val.(string)
		}

		val, err = SetupDiGetDeviceRegistryProperty(handle, idata, SPDRP_LOCATION_INFORMATION)
		if err == nil {
			dev.LocationInformation = val.(string)
		}

		reg, _ := SetupDiOpenDevRegKey(handle, idata, DICS_FLAG_GLOBAL, 0, DIREG_DEV, windows.KEY_READ)
		dev.reg = reg

		affinityPolicyKey, _ := registry.OpenKey(reg, `Interrupt Management\Affinity Policy`, registry.QUERY_VALUE)
		dev.DevicePolicy = int(GetDWORDuint16Value(affinityPolicyKey, "DevicePolicy"))         // REG_DWORD
		dev.DevicePriority = int(GetDWORDuint16Value(affinityPolicyKey, "DevicePriority"))     // REG_DWORD
		dev.AssignmentSetOverride = GetBinaryValue(affinityPolicyKey, "AssignmentSetOverride") // REG_BINARY
		affinityPolicyKey.Close()

		dev.AssignmentSetOverrideBits = Bits(Uvarint(dev.AssignmentSetOverride))

		messageSignaledInterruptPropertiesKey, _ := registry.OpenKey(reg, `Interrupt Management\MessageSignaledInterruptProperties`, registry.QUERY_VALUE)
		dev.MessageNumberLimit = GetDWORDHexValue(messageSignaledInterruptPropertiesKey, "MessageNumberLimit") // REG_DWORD https://docs.microsoft.com/de-de/windows-hardware/drivers/kernel/enabling-message-signaled-interrupts-in-the-registry
		dev.MsiSupported = int(GetDWORDuint16Value(messageSignaledInterruptPropertiesKey, "MSISupported"))     // REG_DWORD
		messageSignaledInterruptPropertiesKey.Close()

		allDevices = append(allDevices, dev)
	}
	return allDevices
}

func SetupDiGetClassDevs(classGuid *windows.GUID, enumerator *uint16, hwndParent uintptr, flags uint32) (handle DevInfo, err error) {
	r0, _, e1 := syscall.Syscall6(procSetupDiGetClassDevsW.Addr(), 4, uintptr(unsafe.Pointer(classGuid)), uintptr(unsafe.Pointer(enumerator)), uintptr(hwndParent), uintptr(flags), 0, 0)
	handle = DevInfo(r0)
	if handle == DevInfo(windows.InvalidHandle) {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}
