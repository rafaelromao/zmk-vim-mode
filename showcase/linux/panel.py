#!/usr/bin/env python3
"""Transparent Wayland showcase surfaces: layer HUD and an exclusive bottom panel."""

import json
import os
from pathlib import Path
import signal
import subprocess
import sys

# WebKit's DMA-BUF renderer triggers a Wayland protocol error on this NVIDIA host.
# Software composition also preserves the transparent webview background reliably.
os.environ.setdefault("WEBKIT_DISABLE_DMABUF_RENDERER", "1")

import gi

gi.require_version("Gtk", "3.0")
gi.require_version("Gdk", "3.0")
gi.require_version("WebKit2", "4.1")
gi.require_version("GtkLayerShell", "0.1")
from gi.repository import Gdk, GLib, Gtk, GtkLayerShell, WebKit2

SHOW = Path(__file__).resolve().parent.parent
RUN = SHOW / "run"
PANEL_HEIGHT = 96
INSET = 8
WINDOWS = []


def surface(monitor, page, namespace, width, height):
    window = Gtk.Window()
    window.set_app_paintable(True)
    window.set_visual(window.get_screen().get_rgba_visual())
    window.set_default_size(width, height)
    GtkLayerShell.init_for_window(window)
    GtkLayerShell.set_namespace(window, namespace)
    GtkLayerShell.set_monitor(window, monitor)
    GtkLayerShell.set_layer(window, GtkLayerShell.Layer.TOP)
    GtkLayerShell.set_keyboard_mode(window, GtkLayerShell.KeyboardMode.NONE)

    manager = WebKit2.UserContentManager()
    manager.add_style_sheet(WebKit2.UserStyleSheet.new(
        "html, body, #keys { background: transparent !important; }"
        "html, body { overflow: hidden !important; }"
        "#hud, #keys .chip { border-color: transparent !important; }",
        WebKit2.UserContentInjectedFrames.ALL_FRAMES,
        WebKit2.UserStyleLevel.USER, None, None))
    view = WebKit2.WebView.new_with_user_content_manager(manager)
    view.set_background_color(Gdk.RGBA(0, 0, 0, 0))
    view.set_size_request(width, height)
    view.load_uri((SHOW / "hud" / page).as_uri() +
                  f"?ws=ws://127.0.0.1:{os.environ.get('ZMKHUD_PORT', '8766')}")
    window.add(view)
    WINDOWS.append(window)
    return window


def main():
    if not GtkLayerShell.is_supported():
        sys.exit("A Wayland compositor with layer-shell support is required")
    monitors = json.loads(subprocess.check_output(["hyprctl", "monitors", "-j"]))
    info = next((m for m in monitors if not m["name"].startswith("eDP")), monitors[0])
    display = Gdk.Display.get_default()
    # Match output geometry rather than assuming GDK and Hyprland enumeration agree.
    monitor = next((display.get_monitor(i) for i in range(display.get_n_monitors())
                    if (display.get_monitor(i).get_geometry().x,
                        display.get_monitor(i).get_geometry().y) == (info["x"], info["y"])), None)
    if monitor is None:
        sys.exit(f"Cannot locate recording monitor {info['name']} in GDK")

    css = Gtk.CssProvider()
    css.load_from_data(b"window, webview { background-color: transparent; }")
    Gtk.StyleContext.add_provider_for_screen(Gdk.Screen.get_default(), css,
                                            Gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
    windows = json.loads(subprocess.check_output(["hyprctl", "clients", "-j"]))
    tiled = [w for w in windows if w["monitor"] == info["id"] and not w["floating"] and w["mapped"]]
    # Stay inside the client area plus an inset: the blue border remains visible.
    top = min((w["at"][1] - info["y"] for w in tiled), default=info["reserved"][1]) + INSET
    right_edge = max((w["at"][0] + w["size"][0] - info["x"] for w in tiled),
                     default=monitor.get_geometry().width - info["reserved"][2])
    right = monitor.get_geometry().width - right_edge + INSET

    hud = surface(monitor, "index.html", "zmkhud-layer", 598, 392)
    GtkLayerShell.set_anchor(hud, GtkLayerShell.Edge.TOP, True)
    GtkLayerShell.set_anchor(hud, GtkLayerShell.Edge.RIGHT, True)
    # Explicit coordinates include the top bar; ignore other panels' exclusive zones.
    GtkLayerShell.set_exclusive_zone(hud, -1)
    GtkLayerShell.set_margin(hud, GtkLayerShell.Edge.TOP, top)
    GtkLayerShell.set_margin(hud, GtkLayerShell.Edge.RIGHT, right)

    # A full-height right rail gives layer-shell an unambiguous exclusive edge.
    # Keep the visible HUD separate so it retains its compact top-right geometry.
    rail = Gtk.Window()
    rail.set_app_paintable(True)
    rail.set_visual(rail.get_screen().get_rgba_visual())
    rail_width = 598 + right + INSET
    rail.set_size_request(rail_width, 1)
    GtkLayerShell.init_for_window(rail)
    GtkLayerShell.set_namespace(rail, "zmkhud-reserved")
    GtkLayerShell.set_monitor(rail, monitor)
    GtkLayerShell.set_layer(rail, GtkLayerShell.Layer.TOP)
    GtkLayerShell.set_keyboard_mode(rail, GtkLayerShell.KeyboardMode.NONE)
    for edge in (GtkLayerShell.Edge.TOP, GtkLayerShell.Edge.BOTTOM, GtkLayerShell.Edge.RIGHT):
        GtkLayerShell.set_anchor(rail, edge, True)
    GtkLayerShell.set_exclusive_zone(rail, rail_width)
    WINDOWS.append(rail)

    keys = surface(monitor, "keys.html", "zmkhud-keys", 900, PANEL_HEIGHT)
    for edge in (GtkLayerShell.Edge.BOTTOM, GtkLayerShell.Edge.LEFT, GtkLayerShell.Edge.RIGHT):
        GtkLayerShell.set_anchor(keys, edge, True)
    GtkLayerShell.set_exclusive_zone(keys, PANEL_HEIGHT)
    rail.show_all()
    keys.show_all()
    hud.show_all()
    print(f"Panel on {info['name']}: reserved right {rail_width}px, bottom {PANEL_HEIGHT}px; "
          f"HUD inset top={top}, right={right}", flush=True)

    def quit_host(*_):
        Gtk.main_quit()
        return False

    for sig in (signal.SIGINT, signal.SIGTERM):
        GLib.unix_signal_add(GLib.PRIORITY_DEFAULT, sig, quit_host)
    with (RUN / "keyfeed.log").open("w") as output:
        feed = subprocess.Popen([sys.executable, "-u", str(SHOW / "linux" / "keyfeed.py")],
                                stdout=output, stderr=output, start_new_session=True)
    def watch_feed():
        if feed.poll() is not None:
            quit_host()
            return False
        return True
    GLib.timeout_add(250, watch_feed)
    try:
        Gtk.main()
    finally:
        for window in WINDOWS:
            window.destroy()
        # Include journalctl, including after the page's close request exits keyfeed.
        try:
            os.killpg(feed.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        feed.wait(timeout=5)


if __name__ == "__main__":
    main()
