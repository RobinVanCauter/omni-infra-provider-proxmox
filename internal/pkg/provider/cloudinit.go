// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/diskfs/go-diskfs/backend/file"
	"github.com/diskfs/go-diskfs/filesystem/iso9660"
	"github.com/luthermonson/go-proxmox"
)

const (
	cloudInitVolumeIdentifier = "cidata"
	cloudInitISOBlockSize     = 2048
)

func (p *Provisioner) uploadCloudInitISO(ctx context.Context, vm *proxmox.VirtualMachine, storage *proxmox.Storage, device, userdata, metadata, vendordata, networkconfig string) error {
	isoName := fmt.Sprintf(proxmox.UserDataISOFormat, vm.VMID)

	isoPath, err := makeCloudInitISO(isoName, userdata, metadata, vendordata, networkconfig)
	if err != nil {
		return err
	}

	defer func() {
		_ = os.Remove(isoPath)
	}()

	task, err := storage.Upload("iso", isoPath)
	if err != nil {
		return err
	}

	if err = task.WaitFor(ctx, 5); err != nil {
		return err
	}

	_, err = vm.AddTag(ctx, proxmox.MakeTag(proxmox.TagCloudInit))
	if err != nil && !proxmox.IsErrNoop(err) {
		return err
	}

	task, err = vm.Config(ctx,
		proxmox.VirtualMachineOption{
			Name:  device,
			Value: fmt.Sprintf("%s:iso/%s,media=cdrom", storage.Name, isoName),
		},
		proxmox.VirtualMachineOption{
			Name:  "boot",
			Value: fmt.Sprintf("%s;%s", vm.VirtualMachineConfig.Boot, device),
		},
	)
	if err != nil {
		return err
	}

	return task.WaitFor(ctx, 2)
}

func makeCloudInitISO(filename, userdata, metadata, vendordata, networkconfig string) (string, error) {
	isoPath := filepath.Join(os.TempDir(), filename)

	isoFile, err := os.Create(isoPath)
	if err != nil {
		return "", err
	}

	if err = isoFile.Close(); err != nil {
		return "", err
	}

	iso, err := file.OpenFromPath(isoPath, false)
	if err != nil {
		return "", err
	}

	defer func() {
		_ = iso.Close()
	}()

	fs, err := iso9660.Create(iso, 0, 0, cloudInitISOBlockSize, "")
	if err != nil {
		return "", err
	}

	if err = fs.Mkdir("/"); err != nil {
		return "", err
	}

	cifiles := map[string]string{
		"user-data": userdata,
		"meta-data": metadata,
	}
	if vendordata != "" {
		cifiles["vendor-data"] = vendordata
	}

	if networkconfig != "" {
		cifiles["network-config"] = networkconfig
	}

	for name, content := range cifiles {
		rw, openErr := fs.OpenFile("/"+name, os.O_CREATE|os.O_RDWR)
		if openErr != nil {
			return "", openErr
		}

		if _, err = rw.Write([]byte(content)); err != nil {
			return "", err
		}
	}

	if err = fs.Finalize(iso9660.FinalizeOptions{
		RockRidge:        true,
		VolumeIdentifier: cloudInitVolumeIdentifier,
	}); err != nil {
		return "", err
	}

	return isoPath, nil
}
