//go:build linux

package uvc

import (
	"fmt"
	"os"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

func TestV4L2Controls(t *testing.T) {
	old := controlIOCTL
	t.Cleanup(func() { controlIOCTL = old })
	values := map[uint32]int32{panID: 0, tiltID: 0, zoomID: 100}
	controlIOCTL = func(_ uintptr, request uintptr, p unsafe.Pointer) error {
		if request == 0xc0445624 {
			q := (*controlInfo)(p)
			q.Min = -3600
			q.Max = 3600
			q.Step = 360
			q.Default = 0
			return nil
		}
		v := (*controlValue)(p)
		if request == 0xc008561b {
			v.Value = values[v.ID]
			return nil
		}
		if request == 0xc008561c {
			values[v.ID] = v.Value
			return nil
		}
		return unix.EINVAL
	}
	file, err := os.CreateTemp(t.TempDir(), "camera")
	if err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	c, err := Open(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if !c.Capabilities().PanTiltAbsolute || !c.Capabilities().ZoomAbsolute {
		t.Fatal(c.Capabilities())
	}
	pan, tilt, err := c.SetPanTilt(9000, -9000)
	if err != nil || pan != 3600 || tilt != -3600 {
		t.Fatalf("clamp: %d %d %v", pan, tilt, err)
	}
	status, err := c.Status()
	if err != nil || status.Pan.Cur != 3600 || status.Tilt.Cur != -3600 {
		t.Fatalf("status: %+v %v", status, err)
	}
	if _, err := c.Home(); err != nil {
		t.Fatal(err)
	}
	if values[panID] != 0 || values[tiltID] != 0 || values[zoomID] != 0 {
		t.Fatal(values)
	}
}

func TestV4L2UnavailableControls(t *testing.T) {
	old := controlIOCTL
	t.Cleanup(func() { controlIOCTL = old })
	controlIOCTL = func(uintptr, uintptr, unsafe.Pointer) error { return unix.EINVAL }
	c, err := Open("/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if c.Capabilities().Any() {
		t.Fatal("unsupported controls advertised")
	}
	if _, err := c.SetZoom(1); err == nil {
		t.Fatal("unsupported zoom succeeded")
	}
}

func TestV4L2SingleAxisControls(t *testing.T) {
	for _, id := range []uint32{panID, tiltID} {
		t.Run(fmt.Sprintf("%x", id), func(t *testing.T) {
			old := controlIOCTL
			t.Cleanup(func() { controlIOCTL = old })
			var value int32
			controlIOCTL = func(_ uintptr, request uintptr, p unsafe.Pointer) error {
				if request == 0xc0445624 {
					q := (*controlInfo)(p)
					if q.ID != id {
						return unix.EINVAL
					}
					q.Min, q.Max, q.Step = -3600, 3600, 360
					return nil
				}
				v := (*controlValue)(p)
				if v.ID != id {
					t.Fatalf("accessed unsupported control %#x", v.ID)
				}
				if request == 0xc008561c {
					value = v.Value
				} else {
					v.Value = value
				}
				return nil
			}
			c, err := Open("/dev/null")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = c.Close() }()
			if !c.Capabilities().Any() {
				t.Fatal("single absolute axis was not advertised")
			}
			if _, _, err := c.SetPanTilt(1800, -1800); err != nil {
				t.Fatal(err)
			}
			want := int32(1800)
			if id == tiltID {
				want = -1800
			}
			if value != want {
				t.Fatalf("position = %d, want %d", value, want)
			}
			if _, err := c.Home(); err != nil || value != 0 {
				t.Fatalf("home position = %d, error = %v", value, err)
			}
		})
	}
}
