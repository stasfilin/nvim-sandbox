export default {
  branches: ["main"],
  tagFormat: "v${version}",
  plugins: [
    [
      "@semantic-release/commit-analyzer",
      {
        preset: "conventionalcommits",
        releaseRules: [
          { breaking: true, release: "minor" },
          { type: "feat", release: "minor" },
          { type: "fix", release: "patch" },
          { type: "perf", release: "patch" },
        ],
      },
    ],
    [
      "@semantic-release/release-notes-generator",
      { preset: "conventionalcommits" },
    ],
    [
      "@semantic-release/exec",
      {
        verifyReleaseCmd:
          "./scripts/verify_release_version.sh ${nextRelease.version}",
        prepareCmd: "./scripts/prepare_release.sh ${nextRelease.version}",
      },
    ],
    [
      "@semantic-release/git",
      {
        assets: ["VERSION", "Formula/nvim-sandbox.rb"],
        message:
          "chore(release): ${nextRelease.version} [skip ci]\n\n${nextRelease.notes}",
      },
    ],
    [
      "@semantic-release/github",
      {
        assets: [
          { path: "release/nvim-sandbox_macos_amd64.tar.gz", label: "macOS Intel" },
          { path: "release/nvim-sandbox_macos_arm64.tar.gz", label: "macOS ARM64" },
          { path: "release/nvim-sandbox_linux_amd64.tar.gz", label: "Linux amd64" },
          { path: "release/nvim-sandbox_linux_arm64.tar.gz", label: "Linux arm64" },
          { path: "release/nvim-sandbox_amd64.deb", label: "Ubuntu/Debian amd64" },
          { path: "release/nvim-sandbox_arm64.deb", label: "Ubuntu/Debian arm64" },
          { path: "release/SHA256SUMS", label: "SHA-256 checksums" },
        ],
      },
    ],
  ],
};
