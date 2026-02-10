# TeamVault Extension Icons

Placeholder icons for the extension. Replace these with production assets.

## Icon Specifications

| File           | Size    | Usage                           |
|----------------|---------|----------------------------------|
| `icon-16.png`  | 16×16   | Toolbar icon, favicon            |
| `icon-48.png`  | 48×48   | Extension management page        |
| `icon-128.png` | 128×128 | Chrome Web Store, install dialog |

## Design

The icon is a vault/lock symbol using the TeamVault brand color (#10b981 — emerald green) on a dark slate background (#0f172a).

### SVG Source

Use this SVG to generate PNGs at the required sizes:

```svg
<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128">
  <rect width="128" height="128" rx="24" fill="#0f172a"/>
  <g transform="translate(24, 20)" stroke="#10b981" stroke-width="5" fill="none">
    <rect x="4" y="38" width="80" height="50" rx="8" ry="8"/>
    <path d="M20 38V26a24 24 0 0 1 48 0v12"/>
    <circle cx="44" cy="60" r="6" fill="#10b981"/>
    <line x1="44" y1="66" x2="44" y2="76" stroke-linecap="round"/>
  </g>
</svg>
```

### Generating PNGs

Using ImageMagick:

```bash
# Save the SVG above as icon.svg, then:
convert -background none icon.svg -resize 16x16 icon-16.png
convert -background none icon.svg -resize 48x48 icon-48.png
convert -background none icon.svg -resize 128x128 icon-128.png
```

Or using Inkscape:

```bash
inkscape icon.svg -w 16 -h 16 -o icon-16.png
inkscape icon.svg -w 48 -h 48 -o icon-48.png
inkscape icon.svg -w 128 -h 128 -o icon-128.png
```

Or simply open the SVG in any image editor and export at the required sizes.
