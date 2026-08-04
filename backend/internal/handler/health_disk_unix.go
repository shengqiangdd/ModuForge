//go:build !windows

package handler

import "syscall"

func getDiskInfo() (*diskInfo, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(".", &stat); err != nil {
		return nil, err
	}
	freeGB := float64(stat.Bavail) * float64(stat.Bsize) / (1024 * 1024 * 1024)
	totalGB := float64(stat.Blocks) * float64(stat.Bsize) / (1024 * 1024 * 1024)
	return &diskInfo{freeGB: freeGB, totalGB: totalGB}, nil
}
