class NvimSandbox < Formula
  desc "Create project-scoped development containers for Neovim"
  homepage "https://github.com/stasfilin/nvim-sandbox"
  version "0.3.0"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.3.0/nvim-sandbox_macos_arm64.tar.gz"
      sha256 "30e43a1a28d6a787095c9aa9b7fb1cd4d1734b0088c1d41ccd21e4a96db3e6b0"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.3.0/nvim-sandbox_macos_amd64.tar.gz"
      sha256 "51e0e4ed17cfd1dcefe9151566834a4f73caea03f781ba42a1bd03ca199745ac"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.3.0/nvim-sandbox_linux_arm64.tar.gz"
      sha256 "58c6544aba34636a5a4ae93bacfcdb9e2d42ad9196a304eb0eb017f6fcda9543"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.3.0/nvim-sandbox_linux_amd64.tar.gz"
      sha256 "923762d1f8de0f29ebb2787322e508aafdf1f2df0ebe7fd72fc6a9df395287e7"
    end
  end

  def install
    bin.install "nvim-sandbox"
  end

  test do
    assert_match "nvim-sandbox #{version}", shell_output("#{bin}/nvim-sandbox version")
  end
end
