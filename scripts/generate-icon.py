from pathlib import Path

from PIL import Image


ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "app" / "resources" / "codex-tray.ico"
TARGET = ROOT / "assets" / "codex-dataproxy.ico"
ACCENT = (20, 184, 166)


def main() -> None:
    if not SOURCE.exists():
        raise FileNotFoundError(f"Codex icon not found: {SOURCE}")

    icon = Image.open(SOURCE).convert("RGBA")
    alpha = icon.getchannel("A")

    tinted = Image.new("RGBA", icon.size, (*ACCENT, 0))
    tinted.putalpha(alpha)

    sizes = sorted(icon.info.get("sizes", {(256, 256), (64, 64), (48, 48), (32, 32), (24, 24), (16, 16)}), reverse=True)

    TARGET.parent.mkdir(parents=True, exist_ok=True)
    tinted.save(TARGET, format="ICO", sizes=sizes)
    print(f"Generated: {TARGET}")


if __name__ == "__main__":
    main()
