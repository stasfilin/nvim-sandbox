class NvimSandbox < Formula
  desc "Create project-scoped development containers for Neovim"
  homepage "https://github.com/stasfilin/nvim-sandbox"
  version "0.5.0"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.5.0/nvim-sandbox_macos_arm64.tar.gz"
      sha256 "4a52e5477cbe0982768b9c0d2428f5e74b090ff7470977d6c071ff682de1d1bc"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.5.0/nvim-sandbox_macos_amd64.tar.gz"
      sha256 "568f955011a484ab24686f35cdccbedaf1e680c714c26ea0812d880118621c40"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.5.0/nvim-sandbox_linux_arm64.tar.gz"
      sha256 "52ad8b9199ca028cfed0831fc5a4ce0f3e6b1773d3a8d68207617460a5cdcf1b"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.5.0/nvim-sandbox_linux_amd64.tar.gz"
      sha256 "518026bb4f4fcf520b2727e1678d5e2b2cb5184717205ae5d5edfc680dba2860"
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
