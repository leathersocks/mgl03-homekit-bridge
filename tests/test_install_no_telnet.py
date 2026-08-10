from __future__ import annotations

import importlib.util
from pathlib import Path
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "scripts" / "install_no_telnet.py"
SPEC = importlib.util.spec_from_file_location("install_no_telnet", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


class NoTelnetInstallerTests(unittest.TestCase):
    def test_supported_firmware_is_intentionally_narrow(self):
        self.assertTrue(MODULE.supported_firmware("1.5.0_0026"))
        self.assertTrue(MODULE.supported_firmware("1.5.4_0090"))
        self.assertFalse(MODULE.supported_firmware("1.5.5_0001"))
        self.assertFalse(MODULE.supported_firmware("1.4.7_0160"))
        self.assertFalse(MODULE.supported_firmware(None))

    def test_mips_elf_validation(self):
        header = bytearray(20)
        header[:4] = b"\x7fELF"
        header[5] = 1
        header[18:20] = (8).to_bytes(2, "little")
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "bridge"
            path.write_bytes(header)
            MODULE.validate_mips_binary(path)
            header[18:20] = (62).to_bytes(2, "little")
            path.write_bytes(header)
            with self.assertRaisesRegex(ValueError, "not MIPS"):
                MODULE.validate_mips_binary(path)

    def test_ipv4_arguments_reject_shell_text(self):
        self.assertEqual(MODULE.ipv4_address("192.168.10.100"), "192.168.10.100")
        with self.assertRaises(Exception):
            MODULE.ipv4_address("192.168.10.100; reboot")

    def test_bootstrap_and_injection_contain_no_token(self):
        bootstrap = MODULE.build_bootstrap(
            "http://192.168.10.100:8123",
            "http://192.168.10.100:8123/callback/abc",
            "a" * 32,
            "b" * 32,
        )
        self.assertIn(b"install-on-device.sh", bootstrap)
        self.assertNotIn(b"token", bootstrap.lower())

        command = MODULE.build_injection(
            "http://192.168.10.100:8123", "c" * 32
        )
        self.assertIn("install.log", command)
        self.assertTrue(command.endswith("&"))
        self.assertLessEqual(len(command.encode()), 700)

    def test_staged_bundle_contains_only_allowlisted_runtime_files(self):
        header = bytearray(20)
        header[:4] = b"\x7fELF"
        header[5] = 1
        header[18:20] = (8).to_bytes(2, "little")

        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / "project"
            scripts = root / "scripts"
            scripts.mkdir(parents=True)
            bridge = root / "bridge"
            bridge.write_bytes(header)
            for name in ("start.sh", "stop.sh", "cleanup.sh", "startup.sh", "install-on-device.sh"):
                (scripts / name).write_bytes(b"#!/bin/sh\nexit 0\n")

            stage = Path(temp) / "stage"
            stage.mkdir()
            previous_root = MODULE.PROJECT_ROOT
            MODULE.PROJECT_ROOT = root
            try:
                MODULE.prepare_stage(stage, b"fake-openmiio", bridge)
            finally:
                MODULE.PROJECT_ROOT = previous_root

            manifest = (stage / "manifest.txt").read_text(encoding="utf-8")
            self.assertEqual(len(manifest.strip().splitlines()), len(MODULE.ARTIFACTS))
            self.assertNotIn("token", manifest.lower())
            self.assertNotIn("key", manifest.lower())
            self.assertNotIn("config.json", manifest)
            self.assertNotIn("/data/mgl03-homekit/hap", manifest)


if __name__ == "__main__":
    unittest.main()
