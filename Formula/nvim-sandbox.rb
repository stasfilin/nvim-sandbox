class NvimSandbox < Formula
  desc "Create project-scoped development containers for Neovim"
  homepage "https://github.com/stasfilin/nvim-sandbox"
  version "0.3.3"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.3.3/nvim-sandbox_macos_arm64.tar.gz"
      sha256 "2f669078a9e1c37008d8f971a98720a57c3ec2844904186b6bdfb7c78b40cd88"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.3.3/nvim-sandbox_macos_amd64.tar.gz"
      sha256 "4c7c78b72fc5e7702ba248115ece97e87e3a159e4eedd4f929e32d4931f200a5"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.3.3/nvim-sandbox_linux_arm64.tar.gz"
      sha256 "6bbe4d93453d1d49c2640b80f33bd56d2a0d6e3838453ed97c2f5029dff85e0a"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.3.3/nvim-sandbox_linux_amd64.tar.gz"
      sha256 "55f8fd56c9f487cdc58cd1da1d929ab8922050154158893c1480aacfae0b6006"
    end
  end

  def install
    bin.install "nvim-sandbox"
    doc.install "LICENSE", "THIRD_PARTY_NOTICES.md"
  end

  test do
    assert_match "nvim-sandbox #{version}", shell_output("#{bin}/nvim-sandbox version")
  end
end
