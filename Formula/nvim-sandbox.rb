class NvimSandbox < Formula
  desc "Create project-scoped development containers for Neovim"
  homepage "https://github.com/stasfilin/nvim-sandbox"
  version "0.3.2"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.3.2/nvim-sandbox_macos_arm64.tar.gz"
      sha256 "0d4ba2285098bbbfc09373f495bc70ebf2b34dae57e4ff0d0d73f0b67bb60bc6"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.3.2/nvim-sandbox_macos_amd64.tar.gz"
      sha256 "d8d506e2cee39e43dc048f40bf2314ed0d5f82a5e24a794cca049f2ab9ed202e"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.3.2/nvim-sandbox_linux_arm64.tar.gz"
      sha256 "a65ad8aea5129ed9f3d33f6197274a55a6f681dd094b083892a8be57bbbec37c"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.3.2/nvim-sandbox_linux_amd64.tar.gz"
      sha256 "ea4ca24aad38046b0c919481c54cee2f79d04eb3b396a872bf537d02a576143a"
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
