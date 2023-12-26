Sentinel as a CLI
=================

Overview overview overview



User Stories
------------

Such users,

So wow


API (Usage?)
------------

command line interface, args, options



Architecture
------------

-----------
wercker.yml
-----------

This is the main file for describing how to build a project, it lives in the
project's code repository and looks like this::


  # This is the base Image to use, this one is provided by wercker
  # but any valid public Docker image should be acceptable.
  box: wercker/ruby

  # Services are Images that we instantiate to be interacted with by the
  # build Containers. This accepts the same Image types as above, but
  # will probably also have access to some private Images that will not be
  # downloadable by users (so that we can sell access to services).
  services:
      - mies/rethinkdb

  # Build is a conceptual Job (see Jobs below) and a physical job in CoreOS,
  # it will instantiate a new Container from the base Image ("box" above) and
  # run the steps defined inside of it, save the output, commit the changes
  # to a new image and push it to our repository.
  build:
      # Each step is provided to the Container as a Volume, the overall build
      # Job will execute each of these steps in the order given.
      steps:
          # Execute the `bundle-install` step provided by wercker.
          - bundle-install
          # Execute a custom script.
          - script:
              # This name will be displayed in the wercker UI and logs.
              name: middleman build
              # This is the actual code to execute
              code: |
                echo "hello world"
                bundle exec middleman build --verbose

  # Deploy is also a conceptual Job and a physical job in CoreOS, it differs
  # from Build in that semantically we aren't usually running Deploy jobs
  # automatically: they are triggered manually from a successful build.
  # As with all Jobs, this has access to the outputs and images from the Jobs
  # before it in the pipeline (in this case the Build job).
  deploy:
      # By default (?), the deploy Container is based on the committed Image
      # from the build that it has been triggered on, but that can be
      # overriden. Some common overrides would be to use a fresh instance
      # of the base Image (the default in the current system) or to use
      # a premade production snapshot.
      #box: $WERCKER_LAST_BUILD_BOX
      steps:
          # Notable here is that we include additional environment variable
          # definitions for the steps. Step creators may need specific
          # settings, this is where they can be set.
          - s3sync:
              # These settings get exposed to the step under a namespace
              # and prefix, for example this will result in the execution of:
              #   export WERCKER_S3SYNC_KEY_ID="$AWS_ACCESS_KEY_ID"
              # Since this is being executed inside the container, it will
              # use the environment to fill the $AWS_ACCESS_KEY_ID at run time.
              # That environment variable was probably set in the wercker
              # UI and passed into the container prior to this step.
              key_id: $AWS_ACCESS_KEY_ID
              key_secret: $AWS_ACCESS_FOO
              # This would expand to:
              #   export WERCKER_S3SYNC_SOURCE_DIR="build/"
              source_dir: build/
      # After-steps are executed regardless of whether the previous steps have
      # succeeded. They are usually used to notify other systems about build
      # success or failure.
      after-steps:
          - hipchat-notify
              token: $HIPCHAT_TOKEN
              room_id: id
              from-name: name

The `wercker.yml` is expected to be in the root of the project directory.


------
Images
------

Images will now be standardized as Docker images.

We will manage our own repository for pushing user images to as the result of
Jobs.

We will keep images for some amount of time before deleting them.


--------
Services
--------

Services are containers instantiated and linked to our build containers so
that they can be accessed via their public interfaces.

For now, Services are just regular boxes like any other, but we expect to have
private services at some point that are paid upgrades.


----
Jobs
----

Jobs are groupings of Steps executed within the same Container.

The beginnings of a Job are:

 - Environment variables associated with the project and Job provided to the
   job by the user via the wercker UI (or locally if dev).
 - Environment variables provided to the Container:
   - Pass-through the variables provided to the Job by the user.
   - Information about Services that have been linked to the Container.
   - Information about the source code repository.
   - Information about each Step as they are executed.
 - The Image to be used, downloaded if necessary by `wercker`.
 - The source code fetched by `codefetcher`.
 - The step code, downloaded by `wercker`.
 - Read-only Volumes attached to the Container containing source and steps.

The results of a Job are:

 - (Production) Entries in the database about metrics (start, stop, usage, etc).
 - (Production) Logs pushed to log storage.
 - (Production) Event notifications about build results.
 - Any files output to $WERCKER_OUTPUT_DIR within the Container.
   These are usually the tarballs of the things that were built.
 - A new image based on committing the container at the end.
 - (Production) The new image pushed to our repository.

In order to communicate with the appropriate APIs in Production, the proper
command-line flags should be set to enable logging, event notifications, and
so forth, with the keys needed to access those resources.


-------------
Jobs (CoreOS)
-------------

Each Job is an individual execution of `wercker` with all of the information
needed by it passed into the environment via the systemd file.

TODO(termie): Add a template for the systemd job file.


Diagram
-------

::

   core-01
                                                         container-12331 --->
  +----------------------------------------------------+
  |     BUILD_ID=foo  \                                |
  |     BUILD_DIR=/tmp/build/$BUILD_ID  \              |
  |     codefetcher github.com/owner/project  \        |
  +     && wercker build owner/project                 +


                                                         +------------------+
                /tmp/build/$BUILD_ID/                    | src/...          |
                                                         | README           |
                                     source +----------> | wercker.yml      |
                                                         +------------------+

         (wercker fetches step) +-----------+
                                            |
                                            |
                                            |            +------------------+
                                            |            | README           |
                                            v            | ...              |
                                                         | wercker-step.yml |
                                     bundle-install +--> | run.sh           |
                                                         +------------------+


                                                         +------------------+
                                                         |                  |
                                     output <----------+ |project-0.15.0.gem|
                                                         |                  |
                                       +                 +------------------+
                                       |
                                       |
                                       |
                                       |
   s3://build-results/$BUILD_ID  <-----+





                       container-12331
 <--- core-01
                      +---------------------------------------------------+
                      |     export BUILD_ID=foo                           |
                      |     export BUILD_DIR=/tmp/build/$BUILD_ID         |
                      |     export WERCKER_DIR=/mnt/wercker               |
                      +     export GIT_URL=github.com/owner/project       +
                            export OUTPUT_DIR=/tmp/build_output
                            cp -r /mnt/wercker/source $BUILD_DIR/
                            cd $BUILD_DIR/source
                            exec /mnt/wercker/bundle-install/run.sh
                            cp dist/project-0.15.0.gem $OUTPUT_DIR

 +------------------+
 | src/...          |       /mnt/wercker/
 | README           |
 | wercker.yml      | +--------------->  source
 +------------------+




 +------------------+
 | README           |
 | ...              |
 | wercker-step.yml |
 | run.sh           | +--------------->  bundle-install
 +------------------+


 +------------------+
 |                  |
 |project-0.15.0.gem| <---+ /tmp/build_output
 |                  |
 +------------------+
                      +                                                   +
                      |                                                   |
                      |                                                   |
                      +---------------------------------------------------+

                                           +
                                           |
                                           |
 docker://wercker/$BUILD_ID    <-----------+



Build Flow
----------

::

  - Execute bootstrap:
    - Create temporary directory for build
    - Download codefetcher, wercker
    - Fetch code through codefetcher (get code -> wercker-api)
  - Check for wercker.yml
    - If not there, possibly generate a default
  - Parse wercker.yml (parse wercker.yml -> wercker-api)
    - Validate structure
    - Validate options
    - Check if boxes, services and steps specified in the wercker.yml exist.
  - Setup environment (setup environment -> wercker-api)
    - Download boxes
      - Download from wercker registry
      - Download from any other docker registry (only for local development,
        and white listed providers, ie docker's registry)
      - Download from local docker image (only for local development)
    - Download services
      - Same as download boxes
    - Download steps
      - Download from wercker registry
      - Download from remote server (only for local development)
      - Download from local path (only for local development)
    - Download wercker cache
    - Extract steps to temporary directory
    - Execute docker attach on main box:
      - Mount code retrieved earlier as readonly volume
      - Mount steps as readonly volumes
      - Mount wercker cache as readonly volume
      - Mount step output directories
      - Link services through the docker link api
  - Report detcted steps to wercker-api
  - Set environment variables (environment variables -> wercker-api)
    - Generic environment variables:
      - WERCKER="true"
      - BUILD="true"
      - CI="true"
      - WERCKER_BUILD_ID="..."
      - WERCKER_BUILD_URL="..."
      - WERCKER_MAIN_PIPELINE_STARTED="..."
      - WERCKER_GIT_DOMAIN="..."
      - WERCKER_GIT_OWNER="..."
      - WERCKER_GIT_REPOSITORY="..."
      - WERCKER_GIT_BRANCH="..."
      - WERCKER_GIT_COMMIT="..."
      - WERCKER_ROOT="..."
      - WERCKER_SOURCE_DIR="..."
      - WERCKER_OUTPUT_DIR="..."
      - WERCKER_CACHE_DIR="..."
      - WERCKER_PIPELINE_DIR="..."
      - WERCKER_REPORT_DIR="..."
      - WERCKER_STARTED_BY="..."
      - WERCKER_APPLICATION_ID="..."
      - WERCKER_APPLICATION_NAME="..."
      - WERCKER_APPLICATION_OWNER_NAME="..."
      - WERCKER_APPLICATION_URL="..."
    - Environment variables specified in the wercker.yml
    - Environment variables set in wercker-api
  - Execute for each step (and after-steps):
    - Set step environment variables:
      - WERCKER_STEP_ROOT="..."
      - WERCKER_STEP_ID="..."
      - WERCKER_STEP_OWNER="..." <- new, this is handy for introspection
      - WERCKER_STEP_NAME="..."
      - WERCKER_REPORT_NUMBERS_FILE="..."
      - WERCKER_REPORT_MESSAGE_FILE="..."
      - WERCKER_REPORT_ARTIFACTS_DIR="..."
    - source run.sh; echo $?;
    - 'docker commit' current step <- handy to check what happened
                                      between two steps
    - report status to wercker-api
  - Save build output (saving build output -> wercker-api)
    - 'docker push' to our own registry
      - tag with the build_id
      - tag with latest
      - tag with green or red (depending on the build outcome)
    - Fetch wercker cache from container and store
    - Fetch build artifacts from container and store
  - Report build status to wercker-api
  - Post build actions:
    - Force shutdown container
    - Delete all files related to this build



Database Impact
---------------

Shouldn't need to interact with the database directly, all data will be
passed at invocation time. No new data needs to be provided.

When running in production, it needs to report back to the logging and
notification services.

Will attempt to upload artifacts and boxes to S3/otherstorage when keys
are provided.


