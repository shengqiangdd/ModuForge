//go:build windows

package handler

import "golang.org/x/sys/windows"

func getDiskInfo() (*diskInfo, error) {
	dir, err := windows.UTF16PtrFromString(".")
	if err != nil {
		return nil, err
	}
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(dir, &freeBytesAvailable, &totalBytes, &totalFreeBytes); err != nil {
		return nil, err
	}
	const gb = 1024 * 1024 * 1024
	return &diskInfo{
		freeGB:  float64(totalFreeBytes) / gb,
		totalGB: float64(totalBytes) / gb,
	}, nil
}
