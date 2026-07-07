class NvimSandbox < Formula
  desc "Create project-scoped development containers for Neovim"
  homepage "https://github.com/stasfilin/nvim-sandbox"
  version "0.6.1"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.6.1/nvim-sandbox_macos_arm64.tar.gz"
      sha256 "78a1a465178f743e32d0a802295fb4af468f5caee3b4359fb3ac2ecdbe251e0e"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.6.1/nvim-sandbox_macos_amd64.tar.gz"
      sha256 "1f6d6bd1971dc3e93e543674c3d55bfa832565a9f7fae9161271afe41c3ca47a"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.6.1/nvim-sandbox_linux_arm64.tar.gz"
      sha256 "33b1d0eb6fd7aa76a2632a269f1b4f90c0bc34f0637440a3d418b9738761bbe4"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.6.1/nvim-sandbox_linux_amd64.tar.gz"
      sha256 "3287fade4d1234fe2850390117b6361da86b7c484d3ab57b137c9cea6a93352b"
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
