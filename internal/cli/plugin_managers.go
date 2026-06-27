package cli

import "strings"

const nativePackPluginCommand = `nvim --headless -c "lua for _, d in ipairs(vim.fn.glob('/root/.local/share/nvim/site/pack/*/start/*/doc', false, true)) do pcall(vim.cmd, 'helptags ' .. d) end" +qa`

type pluginManagerDefinition struct {
	Name                string
	Label               string
	Command             string
	AttachLocalNvimSite bool
}

var pluginManagers = []pluginManagerDefinition{
	{
		Name:                "native-pack",
		Label:               "Native pack",
		Command:             nativePackPluginCommand,
		AttachLocalNvimSite: true,
	},
	{
		Name:    "lazy.nvim",
		Label:   "lazy.nvim",
		Command: `nvim --headless "+Lazy! sync" +qa`,
	},
	{
		Name:    "packer.nvim",
		Label:   "packer.nvim",
		Command: `nvim --headless -c 'autocmd User PackerComplete quitall' -c 'PackerSync'`,
	},
	{
		Name:    "vim-plug",
		Label:   "vim-plug",
		Command: `nvim --headless "+PlugInstall --sync" +qall`,
	},
}

func pluginManagerByName(name string) (pluginManagerDefinition, bool) {
	switch normalizePluginManagerName(name) {
	case "native-pack":
		name = "native-pack"
	case "lazy.nvim":
		name = "lazy.nvim"
	case "packer.nvim":
		name = "packer.nvim"
	case "vim-plug":
		name = "vim-plug"
	default:
		return pluginManagerDefinition{}, false
	}
	for _, manager := range pluginManagers {
		if manager.Name == name {
			return manager, true
		}
	}
	return pluginManagerDefinition{}, false
}

func normalizePluginManagerName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "native", "native-pack", "pack":
		return "native-pack"
	case "lazy", "lazy.nvim":
		return "lazy.nvim"
	case "packer", "packer.nvim":
		return "packer.nvim"
	case "plug", "vimplug", "vim-plug":
		return "vim-plug"
	case "custom":
		return "custom"
	default:
		return name
	}
}

func pluginManagerChoices() []choice {
	choices := make([]choice, 0, len(pluginManagers)+1)
	for _, manager := range pluginManagers {
		label := manager.Label
		if manager.Name == "native-pack" {
			label = "Native pack (mount site/pack + helptags)"
		}
		choices = append(choices, choice{label: label, value: manager.Command})
	}
	choices = append(choices, choice{label: "Custom command", value: "custom"})
	return choices
}
