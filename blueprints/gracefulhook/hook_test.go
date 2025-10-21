package gracefulhook_test

import (
	"bytes"
	"errors"
	"larenso/cluster_autmation/ratelimiter/gracefulhook"
	"log/slog"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestHookSuccess(t *testing.T) {
	var testAsset [4]int
	var testFn [3]func()
	expected := 1
	for x := range testFn {
		testFn[x] = func() {
			testAsset[x] = expected
		}
	}

	initFn := func() error {
		testAsset[3] = expected
		return nil
	}

	sigs := make(chan os.Signal, 1)
	hook := gracefulhook.New(sigs)

	hook.RegShutdown(testFn[0])
	hook.RegShutdownTime(testFn[1], 100*time.Millisecond)
	hook.RegShutdownTimeWithStartup(initFn, testFn[2], 100*time.Microsecond)

	go func() {
		time.Sleep(time.Second * 1)
		sigs <- syscall.SIGINT
	}()
	hook.Wait()

	for x := range testAsset {
		if testAsset[x] != expected {
			t.Errorf("Shutdown hook didn't change testAsset[%d] value", x)
		}
	}
}

func TestHookFnTimeout(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	slog.SetDefault(logger)

	sigs := make(chan os.Signal, 1)
	hook := gracefulhook.New(sigs)

	fun := func() {
		time.Sleep(2 * time.Second)
	}

	testasset := 0
	expected := 1
	hook.RegShutdown(func() {
		testasset = expected
	})
	hook.RegShutdownTime(fun, 100*time.Millisecond)

	go func() {
		time.Sleep(time.Second * 1)
		sigs <- syscall.SIGINT
	}()
	hook.Wait()

	if !strings.Contains(buf.String(), "Graceful shutdown failed within: 100ms") {
		t.Error("Shotdownhook")
	}
	if testasset != expected {
		t.Errorf("Not all showdown hooks were executed")
	}
}

func TestHookStartFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	slog.SetDefault(logger)

	var testAsset [2]int
	var testFn [2]func()
	expected := 1
	for x := range testFn {
		testFn[x] = func() {
			testAsset[x] = expected
		}
	}

	sigs := make(chan os.Signal, 1)
	hook := gracefulhook.New(sigs)

	hook.RegShutdown(testFn[0])
	hook.RegShutdownTimeWithStartup(func() error {
		return errors.New("Start error")
	}, testFn[1], 100*time.Millisecond)

	hook.Wait()
	if testAsset[0] != expected {
		t.Errorf("Not all showdown hooks were executed")
	}
	if testAsset[1] != expected {
		t.Errorf("Not all showdown hooks were executed")
	}
}
