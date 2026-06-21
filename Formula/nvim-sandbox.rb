class NvimSandbox < Formula
  desc "Create project-scoped development containers for Neovim"
  homepage "https://github.com/stasfilin/nvim-sandbox"
  version "0.2.0"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.2.0/nvim-sandbox_macos_arm64.tar.gz"
      sha256 "2cbc6fc61bb586e1b8a4acf4b38be8f97983a9d6f84cfd733d6f608cbbdf8306"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.2.0/nvim-sandbox_macos_amd64.tar.gz"
      sha256 "fdf8a9e6a57ae4173d279a9186e7ba522798a6e826035197d79871f9e829f84c"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.2.0/nvim-sandbox_linux_arm64.tar.gz"
      sha256 "9c9c2e32294f488a947fb350f6e99809f880ec1ffbb5c1b3053f6cd070f92924"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.2.0/nvim-sandbox_linux_amd64.tar.gz"
      sha256 "056f0585ba47548db8f7f6dfc15f7fa5d341c993e7e0cecf52e5e10c654f8c5a"
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
