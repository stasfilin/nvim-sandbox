class NvimSandbox < Formula
  desc "Create project-scoped development containers for Neovim"
  homepage "https://github.com/stasfilin/nvim-sandbox"
  version "0.4.0"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.4.0/nvim-sandbox_macos_arm64.tar.gz"
      sha256 "4f90144f090d2c72d92330292654ce0fdcdb0f3959e2d5ff038b45b5034f00c0"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.4.0/nvim-sandbox_macos_amd64.tar.gz"
      sha256 "c3de28abbe0d705fbc7400c56b1cf8bd470d92747c1322546159dea8e9823b22"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.4.0/nvim-sandbox_linux_arm64.tar.gz"
      sha256 "003dede52fdd61c38503bcb2ec567753c15b24386a33ece4a251903d5faed94a"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.4.0/nvim-sandbox_linux_amd64.tar.gz"
      sha256 "9d50d9d4ce0873050fce14d9646330bc1de9066b6472400ef0b125c62df5e338"
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
