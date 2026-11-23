package nm

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/godbus/dbus/v5"
	"golang.org/x/exp/slog"

	"github.com/geteduroam/linux-app/internal/nm/base"
)

const (
	activeInterface = base.Interface + ".Connection.Active"
	// see https://networkmanager.dev/docs/api/latest/nm-dbus-types.html#NMActiveConnectionState
	activeStateActivated   uint32 = 2
	activeStateDeactivated uint32 = 4
)

// activateTimeout is the maximum time that we wait for one connection
const activateTimeout = 60 * time.Second

func newBase(path dbus.ObjectPath) (*base.Base, error) {
	b := &base.Base{}
	if err := b.Init(base.Interface, path); err != nil {
		return nil, err
	}
	return b, nil
}

// isActive returns whether or not one of the connections is already activated
func isActive(manager *base.Base, uuids []string) bool {
	var paths []dbus.ObjectPath
	if err := manager.Property(&paths, base.Interface+".ActiveConnections"); err != nil {
		slog.Debug("failed to get the active connections", "error", err)
		return false
	}
	for _, p := range paths {
		active, err := newBase(p)
		if err != nil {
			continue
		}
		var uuid string
		var state uint32
		if active.Property(&uuid, activeInterface+".Uuid") != nil || active.Property(&state, activeInterface+".State") != nil {
			continue
		}
		if state == activeStateActivated && slices.Contains(uuids, uuid) {
			return true
		}
	}
	return false
}

// activateOne activates a connection and waits until it succeeds or fails
func activateOne(ctx context.Context, manager *base.Base, uuid string) error {
	con, err := PreviousCon(uuid)
	if err != nil {
		return err
	}
	var path dbus.ObjectPath
	// "/" lets NetworkManager pick the device and the access point
	err = manager.CallReturn(&path, base.Interface+".ActivateConnection", con.Path(), dbus.ObjectPath("/"), dbus.ObjectPath("/"))
	if err != nil {
		return err
	}
	active, err := newBase(path)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			// stop trying, such that NetworkManager can go back to a network that does work
			if derr := manager.Call(base.Interface+".DeactivateConnection", path); derr != nil {
				slog.Debug("failed to deactivate connection", "error", derr)
			}
			return errors.New("timed out, the username or password could be wrong")
		case <-ticker.C:
			var state uint32
			// NetworkManager removes the object if the activation failed
			if err := active.Property(&state, activeInterface+".State"); err != nil || state == activeStateDeactivated {
				return errors.New("the network refused the connection, the username or password could be wrong")
			}
			if state == activeStateActivated {
				return nil
			}
		}
	}
}

// Activate connects to the first connection that works
// It does nothing if one of them is already activated
func Activate(ctx context.Context, uuids []string) error {
	if len(uuids) == 0 {
		return errors.New("no connection profile has been added yet")
	}
	manager, err := newBase(base.ObjectPath)
	if err != nil {
		return err
	}
	if isActive(manager, uuids) {
		return nil
	}
	var last error
	for _, uuid := range uuids {
		tctx, cancel := context.WithTimeout(ctx, activateTimeout)
		last = activateOne(tctx, manager, uuid)
		cancel()
		if last == nil {
			return nil
		}
		slog.Error("failed to activate connection", "uuid", uuid, "error", last)
	}
	return fmt.Errorf("could not connect: %w", last)
}
