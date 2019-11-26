package libpod

import (
	"os"
	"path/filepath"

	"github.com/containers/libpod/libpod/define"
)

// Creates a new volume
func newVolume(runtime *Runtime) (*Volume, error) {
	volume := new(Volume)
	volume.config = new(VolumeConfig)
	volume.state = new(VolumeState)
	volume.runtime = runtime
	volume.config.Labels = make(map[string]string)
	volume.config.Options = make(map[string]string)
	volume.state.NeedsCopyUp = true

	return volume, nil
}

// teardownStorage deletes the volume from volumePath
func (v *Volume) teardownStorage() error {
	return os.RemoveAll(filepath.Join(v.runtime.config.VolumePath, v.Name()))
}

func (v *Volume) isLocalDriver() bool {
	return v.config.Driver == "local" || v.config.Driver == ""
}

// Volumes with options set, or a filesystem type, or a device to mount need to
// be mounted and unmounted.
func (v *Volume) needsMount() bool {
	return !v.isLocalDriver() || (len(v.config.Options) > 0 && v.config.Driver == define.VolumeDriverLocal)
}

// update() updates the volume state from the DB.
func (v *Volume) update() error {
	if err := v.runtime.state.UpdateVolume(v); err != nil {
		return err
	}
	if !v.valid {
		return define.ErrVolumeRemoved
	}
	return nil
}

// save() saves the volume state to the DB
func (v *Volume) save() error {
	return v.runtime.state.SaveVolume(v)
}
