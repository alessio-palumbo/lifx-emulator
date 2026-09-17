# App icon

`lifx-emulator.png` is the selected artwork: a white monitor containing a multicolored 4×4 light grid, with a dark rounded background and transparent exterior. It was created with the built-in image-generation tool.

`build/appicon.png` is the normalized 1024×1024 PNG used by Wails for macOS and Windows packaging, the Linux window icon, and the frontend header. Wails generates the macOS ICNS and Windows ICO resources.

To normalize updated source artwork on macOS:

```sh
sips -z 1024 1024 assets/icon/lifx-emulator.png --out build/appicon.png
```

Wails only generates `build/windows/icon.ico` when it is missing. Remove that generated file before rebuilding Windows after changing the PNG.

Design prompt: a thin white desktop monitor on a charcoal rounded-square background, containing sixteen rounded tiles in a 4×4 grid. Palette rows: royal blue/azure/cyan/turquoise; violet/purple/magenta/pink; red/coral/orange/amber; yellow/lime/green/mint. Keep geometry crisp, gaps even, and glow restrained. Final preparation removes only the exterior background, preserving the approved artwork and giving it actual transparent alpha.
