class NvimSandbox < Formula
  desc "Create project-scoped development containers for Neovim"
  homepage "https://github.com/stasfilin/nvim-sandbox"
  version "0.3.1"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.3.1/nvim-sandbox_macos_arm64.tar.gz"
      sha256 "685a7f7a68ab4020f3966b6401e68526be9bb727e43d4e81e72eefb1b6b87599"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.3.1/nvim-sandbox_macos_amd64.tar.gz"
      sha256 "08386e44fa79dfcd1074fd92085a20133eca12aa428bb691269640104a992ab5"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.3.1/nvim-sandbox_linux_arm64.tar.gz"
      sha256 "3dac2ae497538efbf2bff35f629c6b4f32beedbb8186af2975e1067cbd4c9021"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.3.1/nvim-sandbox_linux_amd64.tar.gz"
      sha256 "5b9bc0b6b77fabed771f5bdf7c9393730bc4169dea798ba26abb5df9aac2e557"
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
