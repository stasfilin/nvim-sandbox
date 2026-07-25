class NvimSandbox < Formula
  desc "Create project-scoped development containers for Neovim"
  homepage "https://github.com/stasfilin/nvim-sandbox"
  version "0.6.2"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.6.2/nvim-sandbox_macos_arm64.tar.gz"
      sha256 "57fa3479ef73021dac8403cf75c40cb3fb6ff9733a3b3506ff9311559b2a2b98"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.6.2/nvim-sandbox_macos_amd64.tar.gz"
      sha256 "ead2dc6a0606cb20a0e6d3f2e3ad7c1be052d96f61db14fd48f00448cf78af9a"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.6.2/nvim-sandbox_linux_arm64.tar.gz"
      sha256 "7a987701d59ed127c10171036f040424d072e64cc61c80d08a677f3c7b3906d5"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.6.2/nvim-sandbox_linux_amd64.tar.gz"
      sha256 "a79476e247bc0fade181d7c40a77de2304a0fe79c4a7da95ca05de76ef25eb74"
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
