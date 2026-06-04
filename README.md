# cocoon

A small sprite atlas packer. Companion to [larva](https://github.com/johans2/larva).

cocoon takes a directory of `.png` sprites and packs them into a single
atlas image plus a `.json` file mapping each sprite name to its pixel
rect in the atlas.

## Usage

```
cocoon <directory> [flags]

Flags:
  -o <file>   Output atlas image (default: atlas.png)
  -p <px>     Padding between sprites (default: 1)
```

Example:

```
cocoon assets/sprites -o atlas.png
```

writes `atlas.png` and `atlas.json`:

```json
{
  "atlas": "atlas.png",
  "size": [268, 313],
  "sprites": {
    "cowboy_run_0": {
      "rect": [98, 142, 21, 32]
    }
  }
}
```

`rect` is `[x, y, width, height]` in atlas pixels — it maps directly to a
source rectangle for your renderer. Normalized UVs, if you ever need them
in a shader, are `rect / size`.

## Notes

- Only `.png` files directly in the given directory are packed (no recursion).
- Sprite names are file names without the extension; duplicates are an error.
- Sprites are shelf-packed tallest-first into a roughly square atlas.
- Padding (default 1px) prevents neighboring sprites from bleeding into
  each other when sampling near edges.

## Install

Requires [Go](https://go.dev/dl/).

```
./install.sh     # linux
install.bat      # windows
```

Installs to `~/.local/bin`.
