package compat

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/containers/libpod/cmd/podman/common"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/specgen"
	"github.com/containers/libpod/pkg/specgen/generate"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func CreateContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	input := handlers.CreateContainerConfig{}
	createOpts := new(common.ContainerCLIOpts)
	query := struct {
		Name string `schema:"name"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}
	if len(input.HostConfig.Links) > 0 {
		utils.Error(w, utils.ErrLinkNotSupport.Error(), http.StatusBadRequest, errors.Wrapf(utils.ErrLinkNotSupport, "bad parameter"))
	}
	_, err := runtime.ImageRuntime().NewFromLocal(input.Image)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "NewFromLocal()"))
		return
	}

	// DEFAULTS

	// TODO We should probably set things from containers.conf here

	createOpts.ReadOnlyTmpFS = true
	createOpts.Systemd = "true"

	// BASE CONFIG

	createOpts.Hostname = input.Hostname
	createOpts.User = input.User
	createOpts.TTY = input.Tty
	createOpts.Interactive = input.OpenStdin
	createOpts.Env = input.Env
	createOpts.Workdir = input.WorkingDir
	createOpts.StopSignal = input.StopSignal

	if input.StopTimeout != nil {
		if *input.StopTimeout < 0 {
			utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Errorf("must provide a stop timeout above 0, got %d", *input.StopTimeout))
		}
		createOpts.StopTimeout = uint(*input.StopTimeout)
	}

	if input.AttachStdin {
		createOpts.Attach = append(createOpts.Attach, "stdin")
	}
	if input.AttachStdout {
		createOpts.Attach = append(createOpts.Attach, "stdout")
	}
	if input.AttachStderr {
		createOpts.Attach = append(createOpts.Attach, "stderr")
	}

	// TODO: Is this right?
	for vol := range input.Volumes {
		createOpts.Volume = append(createOpts.Volume, vol)
	}

	if input.NetworkDisabled {
		createOpts.Net.Network.NSMode = specgen.NoNetwork
	}

	for k, v := range input.Labels {
		createOpts.Label = append(createOpts.Label, fmt.Sprintf("%s=%s", k, v))
	}

	// TODO: Domainname (what is this?)
	// TODO: ExposedPorts (need to parse back into string)
	// TODO: stdinOnce (I don't think we really support this)
	// TODO: Cmd (pass straight into createconfig?)
	// TODO Healthcheck (break up struct)
	// TODO entrypoint (slice vs string)
	// TODO Mac Address
	// TODO onbuild (do we do anything with this?
	// TODO Shell (build specific?)

	// HOST CONFIG

	createOpts.Volume = append(createOpts.Volume, input.HostConfig.Binds...)
	createOpts.CIDFile = input.HostConfig.ContainerIDFile
	createOpts.Rm = input.HostConfig.AutoRemove
	createOpts.VolumesFrom = input.HostConfig.VolumesFrom
	createOpts.CGroupsNS = string(input.HostConfig.CgroupnsMode)
	createOpts.GroupAdd = input.HostConfig.GroupAdd
	createOpts.IPC = string(input.HostConfig.IpcMode)
	createOpts.OOMScoreAdj = input.HostConfig.OomScoreAdj
	createOpts.PID = string(input.HostConfig.PidMode)
	createOpts.Privileged = input.HostConfig.Privileged
	createOpts.PublishAll = input.HostConfig.PublishAllPorts
	createOpts.ReadOnly = input.HostConfig.ReadonlyRootfs
	createOpts.SecurityOpt = input.HostConfig.SecurityOpt
	createOpts.UTS = string(input.HostConfig.UTSMode)
	createOpts.UserNS = string(input.HostConfig.UsernsMode)

	if input.HostConfig.ShmSize != 0 {
		createOpts.ShmSize = fmt.Sprintf("%d", input.HostConfig.ShmSize)
	}

	// TODO LogConfig
	// TODO NetworkMode
	// TODO PortBindings
	// TODO RestartPolicy
	// TODO VolumeDriver (We don't support this)
	// TODO CapAdd/CapDrop
	// TODO Capabilities (We don't support this yet)
	// TODO DNS/DNSOptions/DNSSearch/ExtraHosts
	// TODO Cgroup (is this cgroupparent?)
	// TODO StorageOpt (We don't support this)
	// TODO Tmpfs (different types)
	// TODO sysctls
	// TODO runtime (this is a global option for us)

	// TODO Resources, Mounts, MaskedPaths, ReadonlyPaths, Init

	// TODO Networking Config - we only support a lot of this for a single
	// CNI network, so this should be simple

	specgen := new(specgen.SpecGenerator)
	specgen.Image = input.Image

	if err := common.FillOutSpecGen(specgen, createOpts, input.Cmd); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "error parsing options"))
	}

	warnings, err := generate.CompleteSpec(r.Context(), runtime, specgen)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "error finalizing container configuration"))
	}

	ctr, err := generate.MakeContainer(r.Context(), runtime, specgen)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "error creating container"))
	}

	response := entities.ContainerCreateResponse{
		ID:       ctr.ID(),
		Warnings: warnings,
	}

	utils.WriteResponse(w, http.StatusCreated, response)
}
