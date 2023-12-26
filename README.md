# wercker - your new favorite dev tool
[![wercker status](https://app.wercker.com/status/febe6e1691586f99d20eb79c6b706aaa/s/master "wercker status")](https://app.wercker.com/project/bykey/febe6e1691586f99d20eb79c6b706aaa)

This is the project for `wercker`, the command-line tool that powers
all the build and deploy jobs for [wercker.com](http://wercker.com), it
runs on your local machine with the help of Docker.

Wercker is designed to increase developer velocity by enabling users to run
and automate their tests and builds, leveraging Docker containers to
provide development environments for multi-service architectures.

Note: While the Wercker team has extensive experience with open source, this
project has been internal for some time and may have some rough edges. We'll
be actively prettying up the codebase and the project, but we wanted to
release it to you as early as possible.

The master branch may be in a broken or unstable state during development.
It is recommended that you download `wercker` through
[the CLI section](http://wercker.com/cli/) on our website, if you're not
contributing to the code base.

## Building wercker

`wercker` is built using Go version 1.5 or greater. If you don't have it
already, you can get it from the
[official download page](https://golang.org/dl/). Once you go installed, set
up your go environment by
[using this guide](https://golang.org/doc/code.html#Organization)

Next, you'll need `govendor` to install the golang dependencies. You can do
this by running:
```
  go get github.com/kardianos/govendor
```

In Go 1.5 you'll need the vendor experiment enabled, so make sure to export
`GO15VENDOREXPERIMENT=1` in your shell (see [Go 1.5 Vendor Experiment](https://docs.google.com/document/d/1Bz5-UB7g2uPBdOx-rw5t9MxJwkfpx90cqG9AFL0JAYo/edit))

In your git checkout ($GOPATH/src/github.com/wercker/wercker), run:
```
   govendor sync
```

This command should download the appropiate dependencies.

Once all that is setup, you should be able to run `go build` and get a working
executable named `wercker`.

Once you've got a working Docker environment, running
```
  ./wercker build
```

should go through the entire build and testing process.

Note: this is the bare minimum to build and contribute to the code base. If you
do not have a local Docker environment you will not be able to run and test
`wercker` properly. You can follow [this guide](https://docs.docker.com/engine/installation/) to install Docker on your machine.

## Reporting Bugs

If you are experiencing bugs running wercker locally, please create an issue
containing the following:

- Which OS are you using?
- Which Docker environment are you using? (Boot2docker, custom, etc)
- Create a gist containing the following information:
  - The entire log when running wercker with the `--debug` log. (ie. `wercker --debug build`)
  - The wercker.yml file that causes the issues.

Please don't file any issue dealing with the usage of steps or unexpected behavior on hosted wercker.

If you are experiencing issues running builds or deploys on hosted wercker,
please do the following:

Try running the build again to see if the error keeps occurring. If it does, turn
on support for the application, and create an issue with the following
information:

- The application owner and application name.
- The ID of the build or deploy that failed.

## Contributing to this repository

Oracle welcomes contributions to this repository from anyone.  Please see [CONTRIBUTING](CONTRIBUTING.md) for details.

## Contact

Join us in our slack room: [![Slack Status](http://werckerpublicslack.herokuapp.com/badge.svg)](http://slack.wercker.com)

## License

`wercker` is under the Apache 2.0 license. See the [LICENSE](LICENSE) file for details.
