# testdata

The [testdata](./) directory contains a couple of test steps:

- [invalid_symlink](invalid_symlink): A step with a symlink which points to a
  non-existing file.
- [plain_step](plain_step): Just a `run.sh` and `run.yml`. The manifest should
  be valid.
- [subdir](subdir): A step containing a sub directory.
- [valid_symlink](valid_symlink): A step with a symlink which points to a
  existing file.

The [step_manifests](./step_manifests) directory contains various step
manifests:

- [invalid.yml](./step_manifests/invalid.yml): An invalid yaml, since it uses
  tabs.
- [valid.yml](./step_manifests/valid.yml): A valid step manifest.
