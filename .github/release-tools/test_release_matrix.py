import argparse
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import yaml

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('release_matrix', Path(__file__).with_name('release_matrix.py'))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ReleaseMatrixTest(unittest.TestCase):
    def setUp(self):
        self.previous = Path.cwd()
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        os.chdir(self.temp.name)
        self.addCleanup(os.chdir, self.previous)
        for name in ('.goreleaser.yaml', '.goreleaser.simple.yaml'):
            shutil.copyfile(ROOT / name, name)
        Path('backend/cmd/server').mkdir(parents=True)
        release.VERSION_FILE.write_text('9.8.7\n')

    def fixture_artifacts(self, simple=False):
        directory = Path('release-input')
        directory.mkdir()
        for target in release.targets(simple):
            name = release.archive_name('9.8.7', target)
            archive = directory / name
            if target['goos'] == 'linux':
                with tarfile.open(archive, 'w:gz') as out:
                    info = tarfile.TarInfo('sub2api')
                    info.size = 7
                    info.mode = 0o755
                    out.addfile(info, io.BytesIO(b'fixture'))
            else:
                archive.write_bytes(b'fixture archive')
            metadata = {'version': '9.8.7', 'sha': 'a' * 40, 'target': target,
                        'archive': name, 'sha256': release.sha256(archive)}
            (directory / f"manifest-{target['goos']}-{target['goarch']}.json").write_text(json.dumps(metadata))
        return argparse.Namespace(input='release-input', version='9.8.7', sha='a' * 40, simple=simple, output='contexts')

    def test_full_and_simple_matrix_match_existing_targets(self):
        full = release.targets()
        self.assertEqual(len(full), 5)
        self.assertNotIn({'goos': 'windows', 'goarch': 'arm64'}, full)
        self.assertEqual(release.targets(True), [{'goos': 'linux', 'goarch': 'amd64'}])

    def test_leaf_keeps_packaging_and_selects_only_one_target(self):
        original = release.config()
        release.generate_config(argparse.Namespace(mode='build', simple=False, goos='darwin', goarch='arm64', output='leaf.yaml'))
        leaf = yaml.safe_load(Path('leaf.yaml').read_text())
        self.assertEqual(leaf['builds'][0]['goos'], ['darwin'])
        self.assertEqual(leaf['builds'][0]['goarch'], ['arm64'])
        self.assertEqual(leaf['builds'][0]['ignore'], [])
        self.assertEqual(leaf['archives'], original['archives'])
        self.assertEqual(leaf['release'], original['release'])
        self.assertFalse(leaf['dockers'])
        flags = leaf['builds'][0]['ldflags']
        self.assertIn('{{ .Env.RELEASE_DATE }}', '\n'.join(flags))
        self.assertIn('-X main.Version={{.Version}}', flags)

    def test_publication_config_has_no_compilation_or_docker_work(self):
        for simple in (False, True):
            with self.subTest(simple=simple):
                original = release.config(simple)
                release.generate_config(argparse.Namespace(mode='publish', simple=simple, output='publisher.yaml'))
                data = yaml.safe_load(Path('publisher.yaml').read_text())
                self.assertTrue(data['builds'][0]['skip'])
                self.assertFalse(data['archives'])
                self.assertFalse(data['dockers'])
                self.assertEqual(data['release']['header'], original['release']['header'])
                self.assertEqual(data['release']['footer'], original['release']['footer'])
                if simple:
                    self.assertTrue(data['checksum']['disable'])
                    self.assertTrue(data['release']['skip_upload'])
                else:
                    self.assertEqual(data['checksum']['extra_files'], data['release']['extra_files'])

    def test_collect_and_verify_hash_and_source_binding(self):
        args = self.fixture_artifacts()
        release.verify(args)
        file = next(Path(args.input).glob('*.tar.gz'))
        file.write_bytes(b'corrupted')
        with self.assertRaisesRegex(ValueError, 'checksum mismatch'):
            release.verify(args)

    def test_missing_extra_and_wrong_commit_artifacts_are_rejected(self):
        args = self.fixture_artifacts(True)
        args.sha = 'b' * 40
        with self.assertRaises(ValueError):
            release.verify(args)
        args.sha = 'a' * 40
        Path('release-input/unexpected').write_text('not an asset')
        with self.assertRaises(ValueError):
            release.verify(args)
        Path('release-input/unexpected').unlink()
        next(Path('release-input').glob('*.tar.gz')).unlink()
        with self.assertRaises(FileNotFoundError):
            release.verify(args)

    def test_linux_context_preserves_binary_executable_mode(self):
        args = self.fixture_artifacts()
        Path('Dockerfile.goreleaser').write_text('FROM scratch\nCOPY sub2api /sub2api\n')
        Path('deploy').mkdir()
        Path('deploy/docker-entrypoint.sh').write_text('#!/bin/sh\nexec /app/sub2api\n')
        Path('backend/resources').mkdir()
        Path('backend/resources/data').write_text('fixture')
        release.contexts(args)
        for arch in ('amd64', 'arm64'):
            binary = Path('contexts') / arch / 'sub2api'
            self.assertEqual(binary.read_bytes(), b'fixture')
            self.assertEqual(binary.stat().st_mode & 0o777, 0o755)

    def test_plan_requires_a_tag_for_publication(self):
        args = argparse.Namespace(ref='main', dry_run=False, simple=False)
        with patch.object(subprocess, 'check_output', return_value='a' * 40 + '\n'):
            with self.assertRaisesRegex(ValueError, 'version tag'):
                release.plan(args)
        args.ref = 'v9.8.7'
        with patch.object(subprocess, 'check_output', side_effect=['a' * 40 + '\n', 'b' * 40 + '\n']):
            with self.assertRaisesRegex(ValueError, 'does not match'):
                release.plan(args)

    def test_dry_run_plan_resolves_matrix_without_a_new_tag(self):
        with patch.dict(os.environ, {'GITHUB_OUTPUT': 'outputs', 'GITHUB_REPOSITORY_OWNER': 'ExampleOwner'}), patch.object(subprocess, 'check_output', return_value='a' * 40 + '\n'):
            release.plan(argparse.Namespace(ref='feature/matrix', dry_run=True, simple=False))
        output = dict(line.split('=', 1) for line in Path('outputs').read_text().splitlines())
        self.assertEqual(output['dry_run'], 'true')
        self.assertEqual(output['owner_lower'], 'exampleowner')
        self.assertEqual(len(json.loads(output['matrix'])['include']), 5)

    def test_docker_commands_do_not_publish_during_dry_run(self):
        fake_bin = Path('bin')
        fake_bin.mkdir()
        docker = fake_bin / 'docker'
        docker.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$DOCKER_LOG"\n')
        docker.chmod(0o755)
        env = {**os.environ, 'PATH': str(fake_bin.resolve()) + os.pathsep + os.environ['PATH'],
               'DOCKER_LOG': str(Path('docker.log').resolve()), 'RUNNER_TEMP': self.temp.name,
               'RELEASE_VERSION': '9.8.7', 'RELEASE_SHA': 'a' * 40, 'GITHUB_REPOSITORY': 'ExampleOwner/sub2api',
               'DRY_RUN': 'true', 'SIMPLE_RELEASE': 'false', 'DOCKERHUB_USERNAME': 'skip'}
        subprocess.run(['bash', str(ROOT / '.github/release-tools/release-images.sh')], env=env, check=True)
        log = Path('docker.log').read_text()
        self.assertEqual(log.count('buildx build'), 2)
        self.assertIn('linux/arm64', log)
        self.assertNotIn('--push', log)
        self.assertNotIn('imagetools', log)
        self.assertNotIn('skip/sub2api', log)
        self.assertIn('ghcr.io/exampleowner/sub2api', log)


    def test_published_full_and_simple_image_tags(self):
        fake_bin = Path('bin')
        fake_bin.mkdir()
        docker = fake_bin / 'docker'
        docker.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$DOCKER_LOG"\n')
        docker.chmod(0o755)
        for simple in (False, True):
            with self.subTest(simple=simple):
                log_path = Path(f'docker-{simple}.log').resolve()
                env = {**os.environ, 'PATH': str(fake_bin.resolve()) + os.pathsep + os.environ['PATH'],
                       'DOCKER_LOG': str(log_path), 'RUNNER_TEMP': self.temp.name,
                       'RELEASE_VERSION': '9.8.7', 'RELEASE_SHA': 'a' * 40, 'GITHUB_REPOSITORY': 'ExampleOwner/sub2api',
                       'DRY_RUN': 'false', 'SIMPLE_RELEASE': str(simple).lower(), 'DOCKERHUB_USERNAME': 'fixturehub'}
                subprocess.run(['bash', str(ROOT / '.github/release-tools/release-images.sh')], env=env, check=True)
                log = log_path.read_text()
                self.assertIn('--push', log)
                self.assertEqual(log.count('buildx build'), 1 if simple else 2)
                if simple:
                    self.assertNotIn('fixturehub', log)
                    self.assertNotIn('imagetools', log)
                    self.assertIn('ghcr.io/exampleowner/sub2api:latest', log)
                else:
                    self.assertEqual(log.count('imagetools create'), 2)
                    self.assertIn('fixturehub/sub2api:9.8', log)
                    self.assertIn('ghcr.io/exampleowner/sub2api:9', log)

    def test_goreleaser_configs_embed_the_release_version(self):
        for name in ('.goreleaser.yaml', '.goreleaser.simple.yaml'):
            with self.subTest(name=name):
                data = yaml.safe_load((ROOT / name).read_text())
                self.assertIn('-X main.Version={{.Version}}', data['builds'][0]['ldflags'])

    def test_version_sync_skips_fork_releases_and_this_repository(self):
        script = ROOT / '.github/release-tools/version_sync.py'
        cases = (
            ({}, 1),
            ({'GITHUB_REPOSITORY': 'Marco9442/sub2api'}, 1),
            ({'RELEASE_VERSION': '0.2.21'}, 1),
            ({'GITHUB_REPOSITORY': 'Marco9442/sub2api', 'RELEASE_VERSION': '0.2.21'}, 2),
            ({'GITHUB_REPOSITORY': 'Wei-Shaw/sub2api', 'RELEASE_VERSION': '0.2.21-fork.1'}, 2),
            ({'GITHUB_REPOSITORY': 'Wei-Shaw/sub2api', 'RELEASE_VERSION': '0.2.21'}, 0),
        )
        for extra, expected in cases:
            with self.subTest(extra=extra):
                env = {'PATH': os.environ['PATH'], **extra}
                proc = subprocess.run(['python3', str(script)], env=env, capture_output=True, text=True)
                self.assertEqual(proc.returncode, expected, proc.stderr)

    def test_release_workflow_guards_fork_version_sync(self):
        text = (ROOT / '.github/workflows/release.yml').read_text()
        self.assertIn("FORK_RELEASE: ${{ contains(needs.prepare.outputs.version, '-fork.') }}", text)
        sync = text.split('sync-version-file:', 1)[1]
        self.assertIn('actions: read', sync)
        self.assertIn('$RUNNER_TEMP/release-tools/version_sync.py', sync)
        self.assertLess(sync.index('version_sync.py'), sync.index('echo "$VERSION" > backend/cmd/server/VERSION'))
        self.assertIn('skip=true', sync)
        self.assertIn("steps.version_sync.outputs.skip != 'true'", sync)

    def write_fake_docker(self):
        fake_bin = Path('bin')
        fake_bin.mkdir(exist_ok=True)
        docker = fake_bin / 'docker'
        docker.write_text('''#!/usr/bin/env python3
import json, os, sys
with open(os.environ["DOCKER_LOG"], "a") as fh:
    fh.write(" ".join(sys.argv[1:]) + "\\n")
if "inspect" not in sys.argv:
    sys.exit(0)
ref = sys.argv[-1]
fixture = json.loads(open(os.environ["IMAGE_FIXTURE"]).read())
entry = fixture.get(ref, {"status": "absent"})
status = entry.get("status", "present")
if status == "absent":
    print("manifest unknown", file=sys.stderr)
    sys.exit(1)
if status == "empty":
    sys.exit(0)
if status == "error":
    print(entry.get("text", "denied"), file=sys.stderr)
    sys.exit(entry.get("code", 1))
json.dump(entry["body"], sys.stdout)
''')
        docker.chmod(0o755)
        return fake_bin

    def image_env(self, name, version, simple, fixture, fork_release='false', dry_run='false'):
        fake_bin = self.write_fake_docker()
        fixture_path = Path(name + '.json').resolve()
        fixture_path.write_text(json.dumps(fixture))
        log_path = Path(name + '.log').resolve()
        env = os.environ.copy()
        env.update({
            'PATH': str(fake_bin.resolve()) + os.pathsep + os.environ['PATH'],
            'DOCKER_LOG': str(log_path),
            'RUNNER_TEMP': self.temp.name,
            'RELEASE_VERSION': version,
            'RELEASE_SHA': 'a' * 40,
            'GITHUB_REPOSITORY': 'ExampleOwner/sub2api',
            'DRY_RUN': dry_run,
            'SIMPLE_RELEASE': 'true' if simple else 'false',
            'DOCKERHUB_USERNAME': 'fixturehub',
            'IMAGE_FIXTURE': str(fixture_path),
        })
        if fork_release is None:
            env.pop('FORK_RELEASE', None)
        else:
            env['FORK_RELEASE'] = fork_release
        return env, log_path

    def run_images(self, env):
        return subprocess.run(
            ['bash', str(ROOT / '.github/release-tools/release-images.sh')],
            env=env, capture_output=True, text=True,
        )

    def tag_names(self, log):
        names = []
        for line in log.splitlines():
            parts = line.split()
            for index, part in enumerate(parts[:-1]):
                if part == '--tag':
                    names.append(parts[index + 1].rsplit(':', 1)[-1])
        return names

    def expected_refs(self, version, simple):
        registries = ['ghcr.io/exampleowner/sub2api']
        arches = ['amd64']
        if not simple:
            registries.append('fixturehub/sub2api')
            arches.append('arm64')
        refs = []
        for registry in registries:
            refs.append(f'{registry}:{version}')
            refs.extend(f'{registry}:{version}-{arch}' for arch in arches)
        return refs, arches

    def present_image(self, arches, sha='a' * 40, source='https://github.com/ExampleOwner/sub2api', labels_key='Labels', extra=None):
        manifests = [{'platform': {'os': 'linux', 'architecture': arch}} for arch in arches]
        manifests.append({'platform': {'os': 'unknown', 'architecture': 'unknown'}})
        if extra:
            manifests.extend(extra)
        body = {'manifest': {'manifests': manifests}}
        if len(arches) == 1:
            body['image'] = {
                'os': 'linux',
                'architecture': arches[0],
                'config': {labels_key: {
                    'org.opencontainers.image.revision': sha,
                    'org.opencontainers.image.source': source,
                }},
            }
        return {'status': 'present', 'body': body}

    def matching_fixture(self, version, simple):
        refs, arches = self.expected_refs(version, simple)
        fixture = {}
        for ref in refs:
            if ref.endswith('-amd64'):
                fixture[ref] = self.present_image(['amd64'])
            elif ref.endswith('-arm64'):
                fixture[ref] = self.present_image(['arm64'], labels_key='labels')
            else:
                fixture[ref] = self.present_image(arches)
        return fixture

    def test_fork_releases_omit_floating_tags(self):
        cases = (
            ('version', '0.2.21-fork.1', None, False),
            ('version-simple', '0.2.21-fork.1', None, True),
            ('flag', '9.8.7', 'true', False),
            ('flag-simple', '9.8.7', 'true', True),
        )
        for name, version, fork_release, simple in cases:
            with self.subTest(name=name):
                env, log_path = self.image_env(name, version, simple, {}, fork_release)
                proc = self.run_images(env)
                self.assertEqual(proc.returncode, 0, proc.stderr)
                log = log_path.read_text()
                names = self.tag_names(log)
                self.assertIn(version, names)
                self.assertIn(f'{version}-amd64', names)
                if not simple:
                    self.assertIn(f'{version}-arm64', names)
                    self.assertEqual(log.count('imagetools create'), 2)
                else:
                    self.assertNotIn('fixturehub', log)
                    self.assertEqual(log.count('imagetools create'), 0)
                self.assertNotIn('latest', names)
                self.assertNotIn(version.split('.')[0], names)
                self.assertNotIn('.'.join(version.split('.')[:2]), names)

    def test_fork_dry_run_does_not_inspect_or_publish(self):
        env, log_path = self.image_env('fork-dry', '0.2.21-fork.1', True, {}, None, dry_run='true')
        proc = self.run_images(env)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        log = log_path.read_text()
        self.assertNotIn('inspect', log)
        self.assertNotIn('--push', log)
        self.assertNotIn('latest', self.tag_names(log))

    def test_fork_reuses_complete_matching_images(self):
        for simple in (False, True):
            with self.subTest(simple=simple):
                version = '0.2.21-fork.1'
                env, log_path = self.image_env(
                    f'reuse-{simple}', version, simple, self.matching_fixture(version, simple), None)
                proc = self.run_images(env)
                self.assertEqual(proc.returncode, 0, proc.stderr)
                log = log_path.read_text()
                self.assertNotIn('--push', log)
                self.assertNotIn('buildx build', log)
                self.assertIn('imagetools inspect', log)

    def test_fork_refuses_partial_mismatched_or_unreadable_images(self):
        version = '0.2.21-fork.1'
        refs, _ = self.expected_refs(version, False)
        missing = self.matching_fixture(version, False)
        missing.pop(refs[-1])
        mismatched = self.matching_fixture(version, False)
        mismatched[refs[-1]] = self.present_image(['arm64'], sha='b' * 40)
        wrong_source = self.matching_fixture(version, False)
        wrong_source[refs[-1]] = self.present_image(
            ['arm64'], source='https://github.com/Wei-Shaw/sub2api')
        extra_platform = self.matching_fixture(version, False)
        extra_platform[refs[-1]] = self.present_image(
            ['arm64'], extra=[{'platform': {'os': 'linux', 'architecture': 'amd64'}}])
        empty = {ref: {'status': 'empty'} for ref in refs}
        denied = {ref: {'status': 'error', 'text': 'denied'} for ref in refs}
        cases = {
            'partial': missing,
            'sha': mismatched,
            'source': wrong_source,
            'platform': extra_platform,
            'empty': empty,
            'denied': denied,
        }
        for name, fixture in cases.items():
            with self.subTest(name=name):
                env, log_path = self.image_env(name, version, False, fixture, None)
                proc = self.run_images(env)
                self.assertNotEqual(proc.returncode, 0, proc.stdout + proc.stderr)
                self.assertNotIn('--push', log_path.read_text())


if __name__ == '__main__':
    unittest.main()
