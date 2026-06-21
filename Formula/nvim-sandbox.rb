class NvimSandbox < Formula
  desc "Create project-scoped development containers for Neovim"
  homepage "https://github.com/stasfilin/nvim-sandbox"
  version "0.1.0"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.1.0/nvim-sandbox_macos_arm64.tar.gz"
      sha256 "1c9096fca9400c7a2d4a1ee804ef3ad7277db090e391a4b2b6ddfaff65577305"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.1.0/nvim-sandbox_macos_amd64.tar.gz"
      sha256 "ead804e19908f0a3bd0fe4eb766f8673213320ea7fe7a660f0767cfc2e42516b"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.1.0/nvim-sandbox_linux_arm64.tar.gz"
      sha256 "50996035c0ed7b21eb5b1fc68af3a0b26c87215f2fe5e6de073d0b46cc6e4664"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.1.0/nvim-sandbox_linux_amd64.tar.gz"
      sha256 "f40da886906e6625673ceed5a89e1a0d3601fe73475e6c5180aa349d402f6ae6"
    end
  end

  def install
    bin.install "nvim-sandbox"
  end

  test do
    assert_match "nvim-sandbox #{version}", shell_output("#{bin}/nvim-sandbox version")
  end
end
