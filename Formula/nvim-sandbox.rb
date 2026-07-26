class NvimSandbox < Formula
  desc "Create project-scoped development containers for Neovim"
  homepage "https://github.com/stasfilin/nvim-sandbox"
  version "0.7.0"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.7.0/nvim-sandbox_macos_arm64.tar.gz"
      sha256 "c23ab5a587b569c843ac936f37ad33ec585803a234ad7c71d71a658c5b612b61"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.7.0/nvim-sandbox_macos_amd64.tar.gz"
      sha256 "3336d640341ee794da4e561e32723cf9c9b6786bf1f4fff2ee4101a2be69a0a9"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.7.0/nvim-sandbox_linux_arm64.tar.gz"
      sha256 "c140cc867f3a5e795899131a36da3281606d5d891b06b2568cd14e7c02ca6e4d"
    end

    on_intel do
      url "https://github.com/stasfilin/nvim-sandbox/releases/download/v0.7.0/nvim-sandbox_linux_amd64.tar.gz"
      sha256 "5109a2024dd08eac25d08a86ad3fee641fff2bcaeebce99d52e58aac81cefb1e"
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
