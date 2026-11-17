package gencontent

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const videoRetention = 24 * time.Hour
const videoStorageLimit int64 = 1 << 30

type videoStore struct {
	directory string
	mu        sync.Mutex
}

func newVideoStore() *videoStore {
	directory := os.Getenv("AISTUDIO_VIDEO_DIR")
	if directory == "" {
		runtimeDir := os.Getenv("AISTUDIO_RUNTIME_DIR")
		if runtimeDir == "" {
			runtimeDir = "/app/runtime/state"
		}
		directory = filepath.Join(runtimeDir, "videos")
	}
	return &videoStore{directory: directory}
}

func (s *videoStore) save(data []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.directory, 0700); err != nil {
		return "", err
	}
	used, err := s.pruneExpired()
	if err != nil {
		return "", err
	}
	if used+int64(len(data)) > videoStorageLimit {
		return "", fmt.Errorf("video storage limit of 1 GiB reached; download with response_format=b64_json")
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	id := "video_" + hex.EncodeToString(random)
	file, err := os.OpenFile(s.path(id), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(s.path(id))
		return "", fmt.Errorf("save video: write=%v close=%v", writeErr, closeErr)
	}
	return id, nil
}

func (s *videoStore) open(id string) (*os.File, error) {
	if !validVideoID(id) {
		return nil, os.ErrNotExist
	}
	file, err := os.Open(s.path(id))
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() || time.Since(info.ModTime()) > videoRetention {
		file.Close()
		return nil, os.ErrNotExist
	}
	return file, nil
}

func (s *videoStore) path(id string) string {
	return filepath.Join(s.directory, id+".mp4")
}

func (s *videoStore) pruneExpired() (int64, error) {
	entries, err := os.ReadDir(s.directory)
	if err != nil {
		return 0, err
	}
	var used int64
	for _, entry := range entries {
		id := strings.TrimSuffix(entry.Name(), ".mp4")
		if !strings.HasSuffix(entry.Name(), ".mp4") || !validVideoID(id) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return 0, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if time.Since(info.ModTime()) > videoRetention {
			if err := os.Remove(s.path(id)); err != nil {
				return 0, err
			}
			continue
		}
		used += info.Size()
	}
	return used, nil
}

func validVideoID(id string) bool {
	if !strings.HasPrefix(id, "video_") || len(id) != 38 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(id, "video_"))
	return err == nil
}
