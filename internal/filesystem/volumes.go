package filesystem

import "github.com/shirou/gopsutil/v4/disk"

type VolumesInfo struct {
	Device     string
	MountPoint string
	FStype     string
	TotalBytes uint64
	Free       uint64
	Used       uint64
}

func ListDrivesInfo() ([]VolumesInfo, error) {
	var volsInfo []VolumesInfo
	disks, err := disk.Partitions(false)
	if err == nil {
		for _, dsk := range disks {
			dskUsage, err := disk.Usage(dsk.Mountpoint)
			if err == nil {
				vol := VolumesInfo{dsk.Device, dsk.Mountpoint, dsk.Fstype,
					dskUsage.Total, dskUsage.Free, dskUsage.Used}
				volsInfo = append(volsInfo, vol)
			} else {
				return nil, err
			}
		}
		return volsInfo, nil
	}
	return nil, err
}
