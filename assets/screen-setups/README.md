# Racing simulator screen setup avatars

Six PNG avatars based on the user's supplied screen-setup selection reference, created with the built-in imagegen tool.

- `triple-screens-v2.png` — corrected three-monitor arrangement with both side screens angled toward the driver.
- `triple-screens.png` — original version, retained for reference.
- `single-screen.png` — single standard monitor.
- `wide-screen.png` — wide flat monitor.
- `ultrawide-v2.png` — corrected ultrawide monitor curving toward the driver.
- `ultrawide.png` — original version, retained for reference.
- `vr.png` — VR headset and seated driver.
- `custom.png` — configurable monitor arrangement and settings cog.

All outputs are 1254 × 1254 PNGs with opaque charcoal backgrounds and sage-green illustrations. The generation prompts requested 1024 × 1024; the tool returned the original 1254 × 1254 files saved here without resizing. The backgrounds are opaque, not transparent.

### triple-screens-v2 correction

Current corrected triple-screen avatar: `triple-screens-v2.png`. Both side screens wrap inward toward the driver. The original `triple-screens.png` is retained. Created with built-in imagegen using the original triple-screen PNG as the edit target.

```text
Use case: precise-object-edit.
Edit the attached triple-monitor racing simulator PNG. Change ONLY the perspective of the two SIDE monitors so that both are angled TOWARD the driver, wrapping around the driver in a concave U-shaped arrangement.
Exact geometry correction: the driver is below the monitors in this rear elevated view. Keep the CENTER monitor unchanged. Keep the two INNER vertical side-panel edges next to the center monitor in their current positions. Move BOTH OUTER vertical side-panel edges DOWNWARD on the image, toward the driver, by roughly 90 pixels on this 1254x1254 canvas. The outer corners must be LOWER than the corresponding inner corners, reversing the existing diagonal slopes.
Left monitor desired quadrilateral corners approximately: outer top (190,463), inner top (444,418), inner bottom (444,628), outer bottom (190,674).
Right monitor desired quadrilateral corners approximately: inner top (810,418), outer top (1064,463), outer bottom (1064,674), inner bottom (810,628).
Thus, reading from left to right, the LEFT panel's top and bottom edges slope UP toward the center; the RIGHT panel's top and bottom edges slope DOWN away from the center. The far-left and far-right edges are closest to the driver. Two matching angled trapezoids, a symmetric immersive wraparound rig. Do not preserve the original outward angle.
Preserve the center monitor, seated driver, helmet, seat, cockpit rails, all colors, thin sage outlines, olive screen fills, charcoal background, image dimensions, overall framing, and all other artwork unchanged. No extra objects, text, arrows, annotations, or labels. Return the corrected PNG.
```

### ultrawide-v2 correction

Current corrected ultrawide avatar: `ultrawide-v2.png`. The screen curves inward with both ends wrapping toward the driver. The original `ultrawide.png` is retained. Created with built-in imagegen using the original ultrawide PNG as the edit target.

```text
Use case: precise-object-edit.
Edit the attached ultrawide racing simulator avatar. Change ONLY the direction of curvature of the ultrawide monitor so that its two ends wrap TOWARD the seated driver below it.
The current image bends the wrong way: both top and bottom edges dip DOWNWARD in the middle, while the left and right tips sit higher. REVERSE that bend. In the corrected image BOTH curved horizontal edges must arch UPWARD at the center and descend toward the left and right ends. The screen's middle is the farthest from the driver, while its outer left and right ends extend downward toward the driver, forming an immersive wraparound display in this elevated rear view.
Precise desired silhouette on the 1254x1254 canvas: retain the overall width from approximately x=110 to x=1142. The top border reaches its HIGHEST point at the center x=627, y=400, then smoothly curves downward to y=450 at BOTH outside ends. The lower border reaches its HIGHEST point at the center x=627, y=620, then smoothly curves downward to y=670 at BOTH outside ends. Rounded corners join the two borders with short vertical side edges. Both long edges have the same curvature direction, like a gently arched continuous ribbon, NOT a smile or bowl. Left half rises as it approaches the center, right half falls as it leaves the center. Keep the band roughly 220 pixels tall throughout.
It must remain EXACTLY ONE seamless curved ultrawide monitor, with blank sage fill and pale thin outline, no panel seams. Preserve the seated driver, helmet, bucket seat, cockpit rails, all colors and line weights, charcoal background, square image dimensions, and overall composition unchanged. The helmet should overlap the center lower screen border as before. No text, arrows, labels, annotations, or extra objects. Return the corrected PNG.
```

## Generation prompts

Each final prompt consists of the common prompt below followed by its variant prompt. The user-provided screenshot was attached as Image 1 for every generation.

### Common prompt

```text
Use case: style-transfer.
Create ONE clean square PNG pictogram/avatar for a racing simulator screen-setup selection interface. The attached screenshot is the visual reference: use the simple sage-green equipment illustrations inside the cards. The output should be only one isolated illustration on a completely flat solid dark charcoal-green background (#242B26), edge to edge. Absolutely no transparency, checkerboard, texture, pattern, noise, gradient, vignette, shadow, lighting or photographic effect anywhere. This should look like crisp clean flat vector art rendered to PNG.
Match the screenshot's restrained simple geometric style: muted sage/olive green fills (#556A4A) and thin clean pale sage outlines (#AFBF99). Use delicate strokes, not thick heavy cartoon contours. All fills are uniform solid colors. No card outline, radio button, text, labels, letters, logos or watermark.
The lower-center subject is a small seated racing-simulator driver seen directly from behind and slightly above, round helmet, simple bucket seat and two slim diagonal cockpit rails flaring toward the bottom. Use the exact same shape, size and position of this cockpit across the avatar family: cockpit occupies the central lower area, from about 48% to 76% of the canvas height, its total width about 32% of canvas. Screen equipment is above the cockpit, around 25%-50% canvas height. Keep generous blank margins. Flat simplified diagram, no complex rig detail. Square 1024x1024 composition.
```

### single-screen

```text
Variant: SINGLE SCREEN. Exactly one flat rectangular monitor in front of the driver. A classic 16:9 rectangle, approximately 40% canvas width and 22.5% canvas height, with subtly rounded corners, plain sage fill and a thin pale outline. Straight edges, no curve. Match the upper-middle reference pictogram.
```

### triple-screens

```text
Variant: TRIPLE SCREENS. Exactly THREE distinct monitors arranged in a shallow wraparound arc in front of the driver. A central flat rectangular 16:9 panel, plus one same-size side panel on each side angled inward toward the seated driver (visible as trapezoids). Three equal monitors, two clear seams, a symmetric panorama. Entire display span approximately 70% of canvas width. Match the upper-left reference pictogram.
```

### wide-screen

```text
Variant: WIDE SCREEN. Exactly ONE flat wide 21:9 monitor centered above the driver, approximately 54% canvas width and 23% canvas height. Straight top and bottom edges, subtly rounded corners, no curvature, no seams. Clearly broader than a classic 16:9 monitor. Match the upper-right reference pictogram.
```

### ultrawide

```text
Variant: ULTRAWIDE. Exactly ONE super-ultrawide 32:9 curved monitor wrapping around the front of the driver, approximately 74% canvas width and 21% canvas height. The screen has gently curved top and bottom edges that visibly indicate a wraparound screen in the elevated rear view, rounded outer corners. One uninterrupted sage panel with NO vertical seams. Make it clearly wider and more curved than the wide-screen avatar. Match the lower-left reference idea.
```

### vr

```text
Variant: VR. No external monitors at all. Keep the same seated driver, bucket seat and diagonal cockpit rails, but the driver wears a clearly recognizable VR headset: broad rounded visor seen from slightly above and behind, with a visible headband and side straps around the helmet/head. Above the cockpit in the display-equipment area, show a larger simplified standalone VR headset pictogram with wide rounded visor, a small nose notch at its bottom center, and an arched head strap, all in the same thin sage-line flat style. This headset is the main distinguishing equipment symbol. No monitor rectangle.
```

### custom

```text
Variant: CUSTOM. Show a configurable asymmetric monitor arrangement above the same centered cockpit: one landscape monitor in the middle and one smaller portrait monitor offset to its right, with a small simple six-tooth settings cog above-left of the main monitor. Thin pale sage outlines, solid sage fills. The small cog is a clear arrangement/settings cue in the same flat illustration style. Overall equipment spans about 62% of canvas width. Clearly distinct from the symmetric three-monitor panorama. No text or symbols inside the screens.
```
