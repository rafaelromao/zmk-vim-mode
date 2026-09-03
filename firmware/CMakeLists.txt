# Copyright (c) 2026 Rafael Romão
# SPDX-License-Identifier: MIT

# The Kconfig symbol already carries the central-or-unibody condition.
target_sources_ifdef(
  CONFIG_ZMK_HID_INDICATOR_CODE_LISTENER
  app
  PRIVATE
  src/hid_indicator_code_listener.c
)
