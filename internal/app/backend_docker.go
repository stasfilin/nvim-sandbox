package app

type dockerLikeBackend struct {
	binary string
	name   string
}

func (b dockerLikeBackend) Name() string { return b.name }

func (b dockerLikeBackend) Build(opts ContainerOptions) error {
	args := []string{b.binary, "build"}
	if b.name == "docker" {
		args = append(args, "--progress", "plain")
	}
	args = append(args, "-f", opts.Dockerfile, "-t", opts.Image, opts.ProjectRoot)
	_, err := run(args)
	return err
}

func (b dockerLikeBackend) ImageStatus(image string) (bool, error) {
	result, err := run([]string{b.binary, "image", "inspect", image})
	if err != nil {
		return false, nil
	}
	return inspectStatus(result.Stdout) != "", nil
}

func (b dockerLikeBackend) Create(opts ContainerOptions) error {
	args := []string{b.binary, "create", "--name", opts.ContainerName}
	args = append(args, "--mount", mountSpec(Mount{Source: opts.ProjectRoot, Target: opts.Workspace, Readonly: opts.Readonly}))
	for _, mount := range opts.ExtraMounts {
		args = append(args, "--mount", mountSpec(mount))
	}
	if opts.Network.Enabled == false {
		args = append(args, "--network", "none")
	} else if opts.Network.Name != "" {
		args = append(args, "--network", opts.Network.Name)
	}
	if opts.Network.Enabled {
		for _, port := range opts.Network.Ports {
			args = append(args, "-p", port)
		}
	}
	args = append(args, "-w", opts.Workspace, opts.Image, "/bin/sh", "-lc", "while sleep 3600; do :; done")
	_, err := run(args)
	return err
}

func (b dockerLikeBackend) Start(name string) error {
	_, err := run([]string{b.binary, "start", name})
	return err
}

func (b dockerLikeBackend) Stop(name string) error {
	_, err := run([]string{b.binary, "stop", name})
	return err
}

func (b dockerLikeBackend) Destroy(name string) error {
	_, err := run([]string{b.binary, "rm", "-f", name})
	return err
}

func (b dockerLikeBackend) Exec(opts ContainerOptions, command []string) (string, error) {
	args := []string{b.binary, "exec", "-w", opts.Workspace, opts.ContainerName}
	args = append(args, command...)
	result, err := run(args)
	return result.Stdout, err
}

func (b dockerLikeBackend) ConnectArgs(opts ContainerOptions, command []string) []string {
	args := []string{b.binary, "exec", "-it", "-w", opts.Workspace, opts.ContainerName}
	return append(args, sandboxEnvCommand(command)...)
}

func (b dockerLikeBackend) Logs(name string) (string, error) {
	result, err := run([]string{b.binary, "logs", name})
	return result.Stdout, err
}

func (b dockerLikeBackend) Status(name string) (string, error) {
	result, err := run([]string{b.binary, "inspect", name})
	return inspectStatus(result.Stdout), err
}
