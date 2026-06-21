import assert from "node:assert/strict";

import config from "../release.config.mjs";

const pluginIndex = (name) =>
  config.plugins.findIndex((plugin) =>
    Array.isArray(plugin) ? plugin[0] === name : plugin === name,
  );

const execIndex = pluginIndex("@semantic-release/exec");
const gitIndex = pluginIndex("@semantic-release/git");
const githubIndex = pluginIndex("@semantic-release/github");

assert.notEqual(execIndex, -1, "exec plugin is required to prepare artifacts");
assert.notEqual(gitIndex, -1, "git plugin is required to persist VERSION");
assert.notEqual(githubIndex, -1, "GitHub plugin is required to publish artifacts");
assert.ok(execIndex < gitIndex, "VERSION must be prepared before it is committed");
assert.ok(gitIndex < githubIndex, "VERSION must be committed before publishing");

const [, gitOptions] = config.plugins[gitIndex];
assert.deepEqual(gitOptions.assets, ["VERSION", "Formula/nvim-sandbox.rb"]);
assert.match(gitOptions.message, /^chore\(release\): \$\{nextRelease\.version\}/);
assert.match(gitOptions.message, /\[skip ci\]/);

console.log("Release configuration persists VERSION and the Homebrew formula before publishing.");
