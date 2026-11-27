"""Starts `concourse-lint lsp` in Sublime Text through the LSP package."""

from __future__ import annotations

import hashlib
import io
import os
import shutil
import stat
import tarfile
import urllib.error
import urllib.request
import zipfile

import sublime
from LSP.plugin import LspPlugin, PluginStartError

BINARY_NAME = "concourse-lint"
BINARY_VARIABLE = "${binary}"

# The release whose binary the package downloads: the version of the
# package itself. A release updates it together with CHANGELOG.md (checked by
# tests/test_plugin.py).
VERSION = "0.2.0"
RELEASES = "https://github.com/mfast99/concourse-lint/releases/download/"
DOWNLOAD_TIMEOUT_SECONDS = 60


def archive_name(version: str) -> str:
    """The release archive for this machine, e.g.
    concourse-lint_0.2.0_darwin_arm64.tar.gz."""
    system = {"osx": "darwin", "linux": "linux", "windows": "windows"}[sublime.platform()]
    machine = {"x64": "amd64", "arm64": "arm64"}[sublime.arch()]
    extension = "zip" if system == "windows" else "tar.gz"
    return "%s_%s_%s_%s.%s" % (BINARY_NAME, version, system, machine, extension)


def binary_file_name() -> str:
    return BINARY_NAME + (".exe" if sublime.platform() == "windows" else "")


def downloaded_binary_path() -> str:
    """Where the downloaded binary of VERSION is kept, in the package
    storage of Sublime Text."""
    return os.path.join(sublime.cache_path(), "Package Storage", "LSP-concourse-lint", VERSION, binary_file_name())


def fetch(url: str) -> bytes:
    with urllib.request.urlopen(url, timeout=DOWNLOAD_TIMEOUT_SECONDS) as response:
        return response.read()


def expected_checksum(checksums: str, name: str) -> str:
    for line in checksums.splitlines():
        parts = line.split()
        if len(parts) == 2 and parts[1] == name:
            return parts[0].lower()
    raise PluginStartError("concourse-lint: %s is not listed in checksums.txt of v%s." % (name, VERSION))


def extract_binary(archive: bytes, name: str) -> bytes:
    """The binary from the top level of a release archive."""
    if name.endswith(".zip"):
        with zipfile.ZipFile(io.BytesIO(archive)) as zip_archive:
            return zip_archive.read(binary_file_name())
    with tarfile.open(fileobj=io.BytesIO(archive), mode="r:gz") as tar_archive:
        member = tar_archive.extractfile(binary_file_name())
        if member is None:
            raise PluginStartError("concourse-lint: %s has no binary." % name)
        return member.read()


def download_binary() -> str:
    """Downloads the binary of VERSION from the GitHub release, checks the
    archive against checksums.txt and keeps the binary. Raises URLError (and
    OSError) when GitHub cannot be reached, PluginStartError when the
    checksum does not match."""
    name = archive_name(VERSION)
    base = "%sv%s/" % (RELEASES, VERSION)
    checksums = fetch(base + "checksums.txt").decode("utf-8")
    archive = fetch(base + name)
    if hashlib.sha256(archive).hexdigest() != expected_checksum(checksums, name):
        raise PluginStartError(
            "concourse-lint: the checksum of %s does not match checksums.txt of v%s; "
            "the download was not used." % (name, VERSION)
        )
    binary = extract_binary(archive, name)
    path = downloaded_binary_path()
    os.makedirs(os.path.dirname(path), exist_ok=True)
    # Written under another name first, so that a broken download never
    # leaves a half binary at the path that is used.
    partial_path = path + ".partial"
    with open(partial_path, "wb") as partial_file:
        partial_file.write(binary)
    os.chmod(partial_path, os.stat(partial_path).st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    os.replace(partial_path, path)
    return path


class ConcourseLint(LspPlugin):
    """The settings file LSP-concourse-lint.sublime-settings holds the
    command and the selector; this class only finds the binary: the command
    setting, else the binary of the package version (downloaded once), else
    concourse-lint on the PATH (when GitHub cannot be reached)."""

    @classmethod
    def on_pre_start_async(cls, context) -> None:
        # The user replaced the command, e.g. with a full path: use it as is.
        if BINARY_VARIABLE not in context.configuration.command:
            return
        binary = downloaded_binary_path()
        if not os.path.isfile(binary):
            try:
                binary = download_binary()
            except (urllib.error.URLError, OSError):
                binary = shutil.which(BINARY_NAME)
        if binary is None:
            raise PluginStartError(
                "concourse-lint could not be downloaded and was not found on the PATH. Install it, "
                'or set "command" in Preferences: LSP-concourse-lint Settings to its full path.'
            )
        context.variables["binary"] = binary


def plugin_loaded() -> None:
    ConcourseLint.register()


def plugin_unloaded() -> None:
    ConcourseLint.unregister()
