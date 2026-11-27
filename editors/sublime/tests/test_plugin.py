"""Unit tests for plugin.py without Sublime Text.

Run from the repository root: make sublime-test
The module LSP.plugin of the LSP package is replaced by a stand-in with the
two names plugin.py imports.
"""

import hashlib
import importlib.util
import io
import os
import re
import sys
import tarfile
import tempfile
import types
import unittest
import urllib.error
from unittest import mock

PACKAGE_DIRECTORY = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


class PluginStartError(Exception):
    pass


class LspPlugin:
    registered = False

    @classmethod
    def register(cls):
        cls.registered = True

    @classmethod
    def unregister(cls):
        cls.registered = False


REPOSITORY_ROOT = os.path.dirname(os.path.dirname(PACKAGE_DIRECTORY))


def load_plugin(cache_path="/nonexistent-cache", platform="linux", arch="x64"):
    lsp_package = types.ModuleType("LSP")
    lsp_plugin = types.ModuleType("LSP.plugin")
    lsp_plugin.LspPlugin = LspPlugin
    lsp_plugin.PluginStartError = PluginStartError
    lsp_package.plugin = lsp_plugin
    sublime = types.ModuleType("sublime")
    sublime.cache_path = lambda: cache_path
    sublime.platform = lambda: platform
    sublime.arch = lambda: arch
    modules = {"LSP": lsp_package, "LSP.plugin": lsp_plugin, "sublime": sublime}
    with mock.patch.dict(sys.modules, modules):
        spec = importlib.util.spec_from_file_location("plugin", os.path.join(PACKAGE_DIRECTORY, "plugin.py"))
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module


def start_context(command):
    configuration = types.SimpleNamespace(command=command)
    return types.SimpleNamespace(configuration=configuration, variables={})


class OnPreStartTest(unittest.TestCase):
    """Without a downloaded binary and without network: the PATH."""

    def setUp(self):
        self.plugin = load_plugin()
        offline = mock.patch.object(
            self.plugin.urllib.request, "urlopen", side_effect=urllib.error.URLError("offline")
        )
        offline.start()
        self.addCleanup(offline.stop)

    def test_binary_from_path(self):
        context = start_context(["${binary}", "lsp"])
        with mock.patch.object(self.plugin.shutil, "which", return_value="/usr/local/bin/concourse-lint") as which:
            self.plugin.ConcourseLint.on_pre_start_async(context)
        which.assert_called_once_with("concourse-lint")
        self.assertEqual(context.variables, {"binary": "/usr/local/bin/concourse-lint"})

    def test_binary_missing(self):
        context = start_context(["${binary}", "lsp"])
        with mock.patch.object(self.plugin.shutil, "which", return_value=None):
            with self.assertRaises(PluginStartError) as raised:
                self.plugin.ConcourseLint.on_pre_start_async(context)
        message = str(raised.exception)
        self.assertIn("concourse-lint", message)
        self.assertIn("PATH", message)
        self.assertIn('"command"', message)

    def test_own_command_is_not_looked_up(self):
        context = start_context(["/opt/concourse-lint", "lsp"])
        with mock.patch.object(self.plugin.shutil, "which") as which:
            self.plugin.ConcourseLint.on_pre_start_async(context)
        which.assert_not_called()
        self.assertEqual(context.variables, {})


def release_archive(binary_name, content):
    """A tar.gz like the release archives: the binary at the top level."""
    buffer = io.BytesIO()
    with tarfile.open(fileobj=buffer, mode="w:gz") as archive:
        for name, data in [(binary_name, content), ("LICENSE", b"license")]:
            info = tarfile.TarInfo(name)
            info.size = len(data)
            info.mode = 0o755
            archive.addfile(info, io.BytesIO(data))
    return buffer.getvalue()


class FakeResponse(io.BytesIO):
    def __enter__(self):
        return self

    def __exit__(self, *details):
        self.close()


class DownloadTest(unittest.TestCase):
    """The binary of the release that matches the package version, from
    GitHub, checked against checksums.txt, kept in the package storage."""

    def setUp(self):
        self.cache = tempfile.mkdtemp()
        self.addCleanup(lambda: __import__("shutil").rmtree(self.cache))
        self.plugin = load_plugin(cache_path=self.cache)
        self.version = self.plugin.VERSION
        self.archive_name = "concourse-lint_%s_linux_amd64.tar.gz" % self.version
        self.archive = release_archive("concourse-lint", b"#!/bin/sh\necho binary\n")

    def serve(self, checksums=None, offline=False):
        if checksums is None:
            checksums = "%s  %s\n" % (hashlib.sha256(self.archive).hexdigest(), self.archive_name)
        base = "https://github.com/mfast99/concourse-lint/releases/download/v%s/" % self.version
        responses = {base + "checksums.txt": checksums.encode(), base + self.archive_name: self.archive}
        requested = []

        def urlopen(url, timeout=None):
            requested.append(url)
            if offline:
                raise urllib.error.URLError("offline")
            return FakeResponse(responses[url])

        return mock.patch.object(self.plugin.urllib.request, "urlopen", side_effect=urlopen), requested

    def binary_path(self):
        return os.path.join(self.cache, "Package Storage", "LSP-concourse-lint", self.version, "concourse-lint")

    def test_download_checks_the_checksum_and_keeps_the_binary(self):
        context = start_context(["${binary}", "lsp"])
        patch, requested = self.serve()
        with patch, mock.patch.object(self.plugin.shutil, "which") as which:
            self.plugin.ConcourseLint.on_pre_start_async(context)
        which.assert_not_called()
        self.assertEqual(context.variables, {"binary": self.binary_path()})
        self.assertEqual(len(requested), 2)
        with open(self.binary_path(), "rb") as binary:
            self.assertEqual(binary.read(), b"#!/bin/sh\necho binary\n")
        self.assertTrue(os.access(self.binary_path(), os.X_OK))

    def test_downloaded_binary_is_used_without_network(self):
        os.makedirs(os.path.dirname(self.binary_path()))
        with open(self.binary_path(), "wb") as binary:
            binary.write(b"present")
        context = start_context(["${binary}", "lsp"])
        patch, requested = self.serve()
        with patch:
            self.plugin.ConcourseLint.on_pre_start_async(context)
        self.assertEqual(requested, [])
        self.assertEqual(context.variables, {"binary": self.binary_path()})

    def test_wrong_checksum_is_an_error(self):
        context = start_context(["${binary}", "lsp"])
        patch, _ = self.serve(checksums="%s  %s\n" % ("0" * 64, self.archive_name))
        with patch, mock.patch.object(self.plugin.shutil, "which", return_value="/usr/bin/concourse-lint"):
            with self.assertRaises(PluginStartError) as raised:
                self.plugin.ConcourseLint.on_pre_start_async(context)
        self.assertIn("checksum", str(raised.exception))
        self.assertFalse(os.path.exists(self.binary_path()))

    def test_offline_falls_back_to_the_path(self):
        context = start_context(["${binary}", "lsp"])
        patch, _ = self.serve(offline=True)
        with patch, mock.patch.object(self.plugin.shutil, "which", return_value="/usr/bin/concourse-lint"):
            self.plugin.ConcourseLint.on_pre_start_async(context)
        self.assertEqual(context.variables, {"binary": "/usr/bin/concourse-lint"})

    def test_archive_name_per_platform(self):
        cases = [
            ("linux", "x64", "concourse-lint_1.2.3_linux_amd64.tar.gz"),
            ("linux", "arm64", "concourse-lint_1.2.3_linux_arm64.tar.gz"),
            ("osx", "x64", "concourse-lint_1.2.3_darwin_amd64.tar.gz"),
            ("osx", "arm64", "concourse-lint_1.2.3_darwin_arm64.tar.gz"),
            ("windows", "x64", "concourse-lint_1.2.3_windows_amd64.zip"),
            ("windows", "arm64", "concourse-lint_1.2.3_windows_arm64.zip"),
        ]
        for platform, arch, want in cases:
            plugin = load_plugin(platform=platform, arch=arch)
            self.assertEqual(plugin.archive_name("1.2.3"), want)


class VersionTest(unittest.TestCase):
    def test_version_is_the_newest_release_in_the_changelog(self):
        """The package downloads the binary of its own version: a release
        that adds a version section to CHANGELOG.md updates VERSION too."""
        # The same rule as scripts/release-version.sh (not called here: on
        # Windows "bash" can be the one of WSL, which cannot read the path).
        with open(os.path.join(REPOSITORY_ROOT, "CHANGELOG.md"), encoding="utf-8") as changelog:
            headings = re.findall(r"^## \[([^\]]*)\]", changelog.read(), re.MULTILINE)
        versions = [heading for heading in headings if heading != "Unreleased"]
        if not versions:
            self.skipTest("CHANGELOG.md has no version section yet")
        self.assertEqual(load_plugin().VERSION, versions[0])


class RegistrationTest(unittest.TestCase):
    def test_plugin_loaded_and_unloaded(self):
        plugin = load_plugin()
        plugin.plugin_loaded()
        self.assertTrue(plugin.ConcourseLint.registered)
        plugin.plugin_unloaded()
        self.assertFalse(plugin.ConcourseLint.registered)


if __name__ == "__main__":
    unittest.main()
