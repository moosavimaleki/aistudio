package browserinterface

import "testing"

func TestMemoryUsesTightestBudget(t *testing.T) {
	for _, test := range []struct {
		host, limit, used string
		want              uint64
	}{
		{"MemAvailable: 2048 kB", "1048576", "786432", 262144},
		{"MemAvailable: 256 kB", "1048576", "0", 262144},
		{"MemAvailable: 2048 kB", "1048576", "2097152", 0},
		{"MemAvailable: 2048 kB", "max", "0", 2097152},
	} {
		files := map[string]string{"/proc/meminfo": test.host, "/sys/fs/cgroup/memory.max": test.limit, "/sys/fs/cgroup/memory.current": test.used}
		if got := memoryAvailable(func(path string) string { return files[path] }); got != test.want {
			t.Fatalf("%+v: got %d", test, got)
		}
	}
}

func TestAdaptiveParkingRetainsBrowsersUntilMemoryIsLow(t *testing.T) {
	f := NewFleet(NewBroker(), Config{ChatGPTRotateEvery: 10, Browsers: []BrowserSpec{{ID: "a", Provider: ProviderChatGPT}, {ID: "b", Provider: ProviderChatGPT}, {ID: "google", Provider: ProviderAIStudio}}})
	for _, s := range f.sessions {
		s.started.Store(true)
	}
	f.memoryAvailable = func() uint64 { return 2 * browserMemoryReserve }
	f.parkOtherChatGPT("b")
	if !f.sessions["a"].started.Load() {
		t.Fatal("healthy memory must keep other browsers warm")
	}
	f.memoryAvailable = func() uint64 { return 0 }
	f.parkOtherChatGPT("b")
	if f.sessions["a"].started.Load() {
		t.Fatal("memory pressure must park idle browser")
	}
	if !f.sessions["b"].started.Load() || !f.sessions["google"].started.Load() {
		t.Fatal("current request and other provider must be preserved")
	}
}
