# NVDA global plugin: capture every spoken utterance to a diffable log file.
# Part of the bellui a11yprobe harness (github.com/trendvidia/fyne #566) — turns
# "does the screen reader say the right thing" into a reproducible artifact.
#
# Writes one line per queued speech sequence:  <unix_ms>\tSPEAK\t<utterance>
# The log path comes from the NVDA_SPEECH_LOG env var (falls back next to this
# file). nvda_speechcapture.ps1 loads this plugin via a scratchpad config and
# merges the SPEAK lines with its own MARK lines (same unix-ms clock) into a
# per-step transcript.
#
# Uses the supported speech.extensions.pre_speechQueued action when present
# (NVDA 2021.1+), and falls back to wrapping speech.speak on older builds.

import os
import time
import globalPluginHandler
import speech


def _logpath():
	p = os.environ.get("NVDA_SPEECH_LOG")
	if p:
		return p
	return os.path.join(os.path.dirname(__file__), "speech.log")


def _seq_text(sequence):
	if not sequence:
		return ""
	parts = []
	for item in sequence:
		if isinstance(item, str):
			s = item.strip()
			if s:
				parts.append(s)
	return " ".join(parts)


class GlobalPlugin(globalPluginHandler.GlobalPlugin):
	def __init__(self):
		super().__init__()
		self._f = open(_logpath(), "a", encoding="utf-8", buffering=1)
		self._write("=== speechcapture loaded ===")
		self._orig_speak = None
		hooked = False
		try:
			from speech.extensions import pre_speechQueued
			pre_speechQueued.register(self._on_queued)
			hooked = True
			self._write("=== hook: pre_speechQueued ===")
		except Exception as e:  # noqa: BLE001
			self._write("=== pre_speechQueued unavailable: %r ===" % (e,))
		if not hooked:
			self._orig_speak = speech.speak

			def _wrap(speechSequence, *a, **k):
				try:
					self._on_queued(speechSequence=speechSequence)
				except Exception:  # noqa: BLE001
					pass
				return self._orig_speak(speechSequence, *a, **k)

			speech.speak = _wrap
			self._write("=== hook: speech.speak wrap ===")

	def _write(self, text):
		try:
			self._f.write("%d\tSPEAK\t%s\n" % (int(time.time() * 1000), text))
		except Exception:  # noqa: BLE001
			pass

	def _on_queued(self, speechSequence=None, **kwargs):
		text = _seq_text(speechSequence)
		if text:
			self._write(text)

	def terminate(self, *a, **k):
		try:
			if self._orig_speak is not None:
				speech.speak = self._orig_speak
			else:
				from speech.extensions import pre_speechQueued
				pre_speechQueued.unregister(self._on_queued)
		except Exception:  # noqa: BLE001
			pass
		try:
			self._write("=== speechcapture terminated ===")
			self._f.close()
		except Exception:  # noqa: BLE001
			pass
		super().terminate(*a, **k)
