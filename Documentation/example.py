import eventlet
eventlet.monkey_patch()

import argparse
import collections
import os
import pprint
import shutil
import uuid

import docker
import yaml


# dev only
PROJECT_DIR = './projects'
BUILD_DIR = './builds'
STEP_DIR = './steps'
CONTAINER_MNT = '/mnt'
CONTAINER_TMP = '/tmp'


class WerckerConfig(dict):
  """Top-level parsed wercker.yml file.

  Contains defaults and validation, services, steps and boxes are
  rich objects as well.
  """

  def __init__(self, data):
    global_options = GlobalOptions(data)
    self.data = data
    self.box = Box(self.data.get('box'))
    self.services = Services(self.data.get('services', []))
    self.build = Build(self.data.get('build', {}), global_options)
    #self.deploy = WerckerDeploy(self.data.get('deploy', {}))


class Box(dict):
  """Metadata about a box."""
  def __init__(self, data):
    self['name'] = data
    self.name = data


class Services(list):
  """Aggregate of wercker services."""
  pass


class GlobalOptions(dict):
  def __init__(self, raw):
    self['source-dir'] = raw.get('source-dir', '')
    self['no-response-timeout'] = raw.get('no-response-timeout', 5)
    self['command-timeout'] = raw.get('command-timeout', 10)


class Step(object):
  def __init__(self, step_id, data, build):
    if '/' in step_id:
      owner, name = step_id.split('/', 2)
      step_id = '%s_%s' % (owner, name)
    else:
      owner = 'wercker'
      name = step_id

    self._owner = owner
    self._name = name

    self.id = step_id
    self.data = data
    self.build = build

  def host_path(self):
    return os.path.join(self.build.host_root(), self.id)

  def guest_path(self):
    return os.path.join(self.build.guest_root(), self.id)

  def mnt_path(self):
    return os.path.join(self.build.mnt_root(), self.id)

  def cwd_path(self):
    return self.build.wercker_root()

  # TODO(termie): we need to fetch the step
  def fetch_step(self):
    path = os.path.abspath(os.path.join(STEP_DIR, self.id))
    shutil.copytree(path, self.host_path())
    return self.host_path()

  def report_dir(self):
    return os.path.join(self.build.report_dir(), self.id)

  def report_numbers_file(self):
    return self.report_dir() + '/numbers.ini'

  def report_message_file(self):
    return self.report_dir() + '/message.txt'

  def report_artifacts_dir(self):
    return self.report_dir() + '/artifacts'

  def _get_properties(self):
    step_yml_path = os.path.join(self.host_path(), 'wercker-step.yml')
    o = collections.OrderedDict()
    if os.path.exists(step_yml_path):
      step_config = yaml.load(open(step_yml_path))
      if 'properties' in step_config:
        for k, v in step_config['properties'].iteritems():
          if k in self.data:
            value = self.data[k]
          else:
            value = v.get('default', '')
          key = 'WERCKER_%s_%s' % (self._name.replace('-', '_'), k)
          o[key.upper()] = value
    print o
    return o

  def env(self):
    props = self._get_properties()
    o = {'WERCKER_STEP_ROOT': self.guest_path(),
         'WERCKER_STEP_ID': self.id,
         'WERCKER_STEP_OWNER': self._owner,
         'WERCKER_STEP_NAME': self._name,
         'WERCKER_REPORT_NUMBERS_FILE': 'unusued',
         'WERCKER_REPORT_MESSAGE_FILE': self.report_message_file(),
         'WERCKER_REPORT_ARTIFACTS_DIR': self.report_artifacts_dir(),
         }

    # Sort our env variables, but leave the order of user-provided and keep
    # them after ours (so that they can reference indirectly).
    env = Env(sorted(o.items()))
    for k, v in props.iteritems():
      env[k] = v

    return env


class ScriptStep(Step):
  def __init__(self, data, build):
    step_id = uuid.uuid4().hex
    super(ScriptStep, self).__init__(step_id, data, build)

  # NOTE(termie): we overload the fetch method to generate a
  #               script instead.
  def fetch_step(self):
    os.makedirs(self.host_path())
    script_path = self.host_path() + '/run.sh'
    content = self._normalize_code(self.data['code'])
    with open(script_path, 'w') as fp:
      fp.write(content)

    return self.host_path()

  def _normalize_code(self, s):
    code = s.split('\n')
    if not code[0].startswith('#!'):
      code.insert(0, '#!/bin/bash -xe')
    return '\n'.join(code)


class Build(object):
  """Collection of build steps and an environment."""

  # Some variables from the current environment will be passed on down
  # to the container if present.
  MIRROR_ENV = ['WERCKER_GIT_DOMAIN',
                'WERCKER_GIT_OWNER',
                'WERCKER_GIT_REPOSITORY',
                'WERCKER_GIT_BRANCH',
                'WERCKER_GIT_COMMIT',
                'WERCKER_STARTED_BY',
                'WERCKER_MAIN_PIPELINE_STARTED',
                'WERCKER_APPLICATION_URL',
                'WERCKER_APPLICATION_ID',
                'WERCKER_APPLICATION_NAME',
                'WERCKER_APPLICATION_OWNER_NAME',
                ]

  _id = None

  def __init__(self, data, global_options):
    self.data = data
    self.global_options = global_options
    self.steps = self._convert_steps(data.get('steps', []))


  def _convert_steps(self, steps):
    """Convert [{step_id: step_content}] to a list of Steps."""
    steps_list = [(d.items()[0][0], d.items()[0][1]) for d in steps]
    steps_list.insert(0, ('wercker-init', {}))
    out = []
    for step_id, step_content in steps_list:
      if step_id == 'script':
        out.append(ScriptStep(step_content, self))
      else:
        out.append(Step(step_id, step_content, self))

    return out

  def host_root(self):
    """The root dir for this build on the host machine."""
    return os.path.abspath('%s/%s' % (BUILD_DIR, self.id()))

  def mnt_root(self):
    """The root dir for where we mount everything on the guest read-only."""
    return '/mnt'

  def guest_root(self):
    """The directory where we copy everything for read-write purposes."""
    return '/pipeline'

  def report_dir(self):
    return self.guest_root() + '/report'

  def wercker_root(self):
    return self.guest_root() + '/source'

  def id(self):
    if not self._id:
      self._id = os.environ.get('WERCKER_BUILD_ID', uuid.uuid4().hex)
    return self._id

  def env(self):
    wercker_root = self.wercker_root()
    env = self._get_passthru_env()
    env.update(self._get_mirror_env())
    o = {'WERCKER': 'true',
         'BUILD': 'true',
         'CI': 'true',
         'WERCKER_BUILD_ID': self.id(),
         'WERCKER_ROOT': wercker_root,
         'WERCKER_SOURCE_DIR': os.path.join(
            wercker_root, self.global_options['source-dir']),
         'WERCKER_CACHE_DIR': '/cache',
         'WERCKER_OUTPUT_DIR': self.guest_root() + '/output',
         'WERCKER_PIPELINE_DIR': self.guest_root(),
         'WERCKER_REPORT_DIR': self.report_dir(),
         'TERM': 'xterm-256color',
         }
    env.update(o)

    # Let's sort our general env alphabetically for easier debugging
    return Env(sorted(env.items()))

  def _get_mirror_env(self):
    o = {}
    for x in self.MIRROR_ENV:
      if x in os.environ:
        o[x] = os.environ.get(x)

    return o

  def _get_passthru_env(self):
    o = {}
    for k, v in os.environ.iteritems():
      if k.startswith('PASSTHRU_'):
        o[k[len('PASSTHRU_'):]] = v

    return o


class Env(collections.OrderedDict):
  template = 'export %s="%s"'
  def to_cmd(self):
    return [self.template % (k, v) for k, v in self.iteritems()]


class Session(object):
  """Run commands in a container and keep track of output."""
  def __init__(self, container_id, d=None):
    self.container_id = container_id
    self.d = d
    self._sent = []
    self._send_idx = 0

  def attach(self):
    ws = self.d.attach_socket(self.container_id,
                              params={'stdin': 1,
                                      'stdout': 1,
                                      'stderr': 1,
                                      'stream': 1},
                              ws=True)
    self.ws = ws

    self._recv_queue = eventlet.Queue(0)
    self._recv_thr = eventlet.spawn(self._start_recv)

  def _start_recv(self):
    line = ''
    while True:
      data = self.ws.recv()
      if data is None:
        eventlet.sleep(10)
        continue
      line += data
      print 'raw', repr(data)
      if '\n' in line:
        parts = line.split('\n')
        line = parts[-1]
        for part in parts[:-1]:
          print "recv", part
          self._recv_queue.put(part)

  def send(self, commands):
    if type(commands) is type(""):
      commands = [commands]
    self._sent.append(commands)
    for cmd in commands:
      print 'send', cmd
      self.ws.send(cmd + '\n')


  def send_checked(self, commands):
    rand_id = uuid.uuid4().hex
    self.send(commands)
    self.send('echo %s $?' % rand_id)

    check = False
    exit_code = None
    recv = []
    while not check:
      line = self.recv().next()
      if not line:
        continue
      line = line.strip()
      if line.startswith('%s ' % rand_id):
        check = True
        exit_code = line[len('%s ' % rand_id):]
      else:
        recv.append(line)
    return (exit_code, recv)


  def recv(self):
    while True:
      yield self._recv_queue.get()


def load_config(path):
  # ...
  parsed_yaml = yaml.load(open(path))
  return WerckerConfig(parsed_yaml)


def maybe_pull_image(box, d=None):
  images = d.images()
  #pprint.pprint(images)

  # If we already have the image, don't pull
  for image in images:
    if (box.name in image['RepoTags']
        or '%s:latest' % box.name in image['RepoTags']):
      return image

  # Didn't find the image, pull it down
  image = d.pull(box.name)
  return image


def create_container(image, box=None, volumes=None, d=None):
  default_params = {
    'stdin_open': True,
    'tty': False,
    'command': '/bin/bash',
    'volumes': volumes and volumes or [],
    'name': 'wercker-build-' + uuid.uuid4().hex
  }
  container = d.create_container(image['Id'], **default_params)
  return container


def cli_build(args):
  project = args.project

  # Set up our build directory, all the things we checkout or
  # supply to the build will come from here
  build_id = uuid.uuid4().hex
  os.environ['WERCKER_BUILD_ID'] = build_id
  build_path = os.path.abspath(os.path.join(BUILD_DIR, build_id))
  os.makedirs(build_path)

  # Get the code and link it in
  project_path = checkout_project(project)
  checkout_path = os.path.join(build_path, 'source')
  shutil.copytree(project_path, checkout_path)

  # Parse the yaml file
  config = load_config(os.path.join(checkout_path, 'wercker.yml'))
  #pprint.pprint(config.data)


  # Download any steps we need
  # TODO(termie): download some steps and link them into build dir
  for step in config.build.steps:
    print step.fetch_step()


  # Build list of volumes to mount
  volume_paths = dict(
      (os.path.join(CONTAINER_MNT, x), os.path.join(build_path, x))
      for x in os.listdir(build_path))
  binds = dict((v, {'bind': k, 'ro': True})
               for k, v in volume_paths.iteritems())

  pprint.pprint(volume_paths.keys())
  pprint.pprint(binds)

  #pprint.pprint(config.box)

  # Execute the build
  d = docker.Client(base_url='tcp://127.0.0.1:4243')
  image = maybe_pull_image(config.box, d=d)

  pprint.pprint(image)

  c = create_container(image, config.box, volumes=volume_paths.keys(), d=d)
  print build_id
  print c

  d.start(c['Id'], binds=binds)

  session = Session(c['Id'], d=d)
  session.attach()


  # General build parts, copy the source and setup the env
  source_mnt = os.path.join(config.build.mnt_root(), 'source')
  source_path = os.path.join(config.build.guest_root(), 'source')

  (exit_code, recv) = session.send_checked(
      'mkdir "%s"' % config.build.guest_root())
  (exit_code, recv) = session.send_checked(
      ['cp -r %s %s' % (source_mnt, source_path)])


  # reporter.step_started('Setup environment')
  (exit_code, recv) = session.send_checked(config.build.env().to_cmd())

  # Copy all the steps
  for step in config.build.steps:
    # We don't want to exit on errors, we want to trap them and know about
    # them.
    (exit_code, recv) = session.send_checked(['set +e'])

    (exit_code, recv) = session.send_checked(
        ['cp -r %s %s' % (step.mnt_path(), step.guest_path())])
    print '%s : %s' % (exit_code, recv)

    (exit_code, recv) = session.send_checked('cd "%s"' % step.cwd_path())

    (exit_code, recv) = session.send_checked(step.env().to_cmd())


    if os.path.exists('%s/%s' % (step.host_path(), 'init.sh')):
      (exit_code, recv) = session.send_checked(
          'source "%s/%s"' % (step.guest_path(), 'init.sh'))
      print '%s : %s' % (exit_code, recv)

    if os.path.exists('%s/%s' % (step.host_path(), 'run.sh')):
      (exit_code, recv) = session.send_checked([
          'chmod +x "%s/%s"' % (step.guest_path(), 'run.sh'),
          'source "%s/%s"' % (step.guest_path(), 'run.sh')
          ])
      print '%s : %s' % (exit_code, recv)


  # Execute the steps
  #for step_id, step in config.build.steps:
  #  if step_id == 'script':
  #    step_id = step['id']

  #  container_path = os.path.join(CONTAINER_TMP, step_id)
  #  init_path = os.path.join(container_path, 'init.sh')
  #  run_path = os.path.join(container_path, 'run.sh')

  #  env_template = 'export WERCKER_%(step_id)s_%(key)s="%(value)s"'
  #  commands = []
  #  commands.append('export WERCKER_STEP_ROOT="%s"' % container_path)
  #  for k, v in step.get('_properties', {}).iteritems():
  #    if k in step:
  #      value = step.get(k)
  #    else:
  #      value = v.get('default', '')

  #    commands.append(env_template % {'step_id': step_id.upper(),
  #                                    'key': k.upper(),
  #                                    'value': value})
  #  commands.append('cd "%s"' % source_path)
  #  if os.path.exists(os.path.join(build_path, step_id, 'init.sh')):
  #    commands.append('source "%s"' % init_path)

  #  if os.path.exists(os.path.join(build_path, step_id, 'run.sh')):
  #    commands.append('chmod +x "%s"' % run_path)
  #    commands.append('source "%s"' % run_path)

  #  (exit_code, recv) = session.send_checked(commands)
  #  print '%s : %s' % (exit_code, recv)


  session.send_checked("echo foo 1>&2")

  i = 0
  for data in session.recv():
    print i, data
    i += 1


def get_cli():
  parser = argparse.ArgumentParser(description="wercker")
  subparsers = parser.add_subparsers()
  parser_build = subparsers.add_parser('build')
  parser_build.set_defaults(func=cli_build)
  parser_build.add_argument('project')
  return parser

if __name__ == '__main__':
  cli = get_cli()
  args = cli.parse_args()

  args.func(args)
