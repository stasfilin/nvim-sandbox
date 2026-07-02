class NvimSandbox < Formula
  desc "Create project-scoped development containers for Neovim"
  homepage "https://github.com/stasfilin/nvim-sandbox"
  version "0.6.0"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.6.0/nvim-sandbox_macos_arm64.tar.gz"
      sha256 "0795ae657100b8d6c530fe8a013c4995bf235ec640165ac02ffc8174b75b5b8b"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.6.0/nvim-sandbox_macos_amd64.tar.gz"
      sha256 "213f15abe8ef5e0fa286173db680144d84669e17b3393f913645c646fca3ffe8"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.6.0/nvim-sandbox_linux_arm64.tar.gz"
      sha256 "e7a616dd2576da2295cadbe3a1d0e7697ef2a82c4c46040abd221d25f7efca2f"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.6.0/nvim-sandbox_linux_amd64.tar.gz"
      sha256 "38329e68e85a13c16b0bd92fcaec21e9ecf48885942dacde8df15ff9a250ccaa"
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
