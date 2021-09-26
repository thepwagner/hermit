package build

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

func CopyVolume(ctx context.Context, src, dst string) error {
	if err := exec.CommandContext(ctx, "/usr/bin/cp", "--reflink=always", src, dst).Run(); err != nil {
		return fmt.Errorf("copying file: %w", err)
	}
	return nil
}

func CreateVolume(ctx context.Context, path string, sizeMB int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	if err := f.Truncate(int64(sizeMB) * 1024 * 1024); err != nil {
		return fmt.Errorf("truncating file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing file: %w", err)
	}
	if err := exec.CommandContext(ctx, "/sbin/mkfs.ext4", "-m", "0", path).Run(); err != nil {
		return fmt.Errorf("creating filesystem: %w", err)
	}
	return nil
}

type MountedVolume string

func MountVolume(ctx context.Context, volume, dir string) (MountedVolume, error) {
	tmpDir, err := ioutil.TempDir(dir, "volume-*")
	if err != nil {
		return "", err
	}
	if err := exec.CommandContext(ctx, "/bin/mount", "-o", "loop,noatime", volume, tmpDir).Run(); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", fmt.Errorf("mounting volume: %w", err)
	}
	return MountedVolume(tmpDir), nil
}

func (m MountedVolume) Path() string {
	return string(m)
}

func (m MountedVolume) Close(ctx context.Context) error {
	mnt := m.Path()
	if err := exec.CommandContext(ctx, "/bin/umount", mnt).Run(); err != nil {
		return err
	}
	return os.Remove(mnt)
}
