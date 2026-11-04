export const PROJECT_ICON_SIZE = 64;
export const MAX_PROJECT_ICON_LENGTH = 64 * 1024;
export const MAX_ICON_FILE_BYTES = 5 * 1024 * 1024;
export const ICON_FILE_TYPES = ['image/png', 'image/jpeg', 'image/gif', 'image/webp', 'image/svg+xml'];

function iconCanvas(): [HTMLCanvasElement, CanvasRenderingContext2D] {
    const canvas = document.createElement('canvas');
    canvas.width = PROJECT_ICON_SIZE;
    canvas.height = PROJECT_ICON_SIZE;
    const context = canvas.getContext('2d');
    if (!context) throw new Error('Icons cannot be drawn in this window.');
    return [canvas, context];
}

function encode(canvas: HTMLCanvasElement): string {
    const icon = canvas.toDataURL('image/png');
    if (!icon.startsWith('data:image/png;base64,')) throw new Error('The icon could not be converted to PNG.');
    if (icon.length > MAX_PROJECT_ICON_LENGTH) throw new Error('The icon is too detailed. Choose a simpler image.');
    return icon;
}

const EMOJI_CANVAS_SIZE = 256;

/** Bounding box of pixels with any opacity in RGBA `data`, or null if all are transparent. */
export function opaqueBounds(data: ArrayLike<number>, width: number, height: number): {x: number; y: number; width: number; height: number} | null {
    let left = width, top = height, right = -1, bottom = -1;
    for (let y = 0; y < height; y++) {
        for (let x = 0; x < width; x++) {
            if (!(data[(y * width + x) * 4 + 3] > 0)) continue;
            if (x < left) left = x;
            if (x > right) right = x;
            if (y < top) top = y;
            if (y > bottom) bottom = y;
        }
    }
    return right < 0 ? null : {x: left, y: top, width: right - left + 1, height: bottom - top + 1};
}

/**
 * Renders an emoji or short text as a square PNG data URL; text uses `color`.
 * The drawn glyphs are trimmed to their visible pixels and scaled to fill the icon.
 */
export function emojiIcon(text: string, color: string): string {
    const symbol = text.trim();
    if (!symbol) throw new Error('Enter an emoji or short text first.');
    const work = document.createElement('canvas');
    work.width = EMOJI_CANVAS_SIZE;
    work.height = EMOJI_CANVAS_SIZE;
    const workContext = work.getContext('2d', {willReadFrequently: true});
    if (!workContext) throw new Error('Icons cannot be drawn in this window.');
    workContext.textAlign = 'center';
    workContext.textBaseline = 'middle';
    workContext.fillStyle = color;
    let size = 192;
    const fontFor = (px: number) => `${px}px "Segoe UI Emoji", "Apple Color Emoji", "Noto Color Emoji", "Segoe UI", sans-serif`;
    workContext.font = fontFor(size);
    while (size > 16 && workContext.measureText(symbol).width > EMOJI_CANVAS_SIZE - 16) {
        size -= 8;
        workContext.font = fontFor(size);
    }
    workContext.fillText(symbol, EMOJI_CANVAS_SIZE / 2, EMOJI_CANVAS_SIZE / 2);
    const bounds = opaqueBounds(workContext.getImageData(0, 0, EMOJI_CANVAS_SIZE, EMOJI_CANVAS_SIZE).data, EMOJI_CANVAS_SIZE, EMOJI_CANVAS_SIZE);
    if (!bounds) throw new Error('This text has no visible characters.');
    const [canvas, context] = iconCanvas();
    context.imageSmoothingQuality = 'high';
    const scale = PROJECT_ICON_SIZE / Math.max(bounds.width, bounds.height);
    const drawWidth = bounds.width * scale;
    const drawHeight = bounds.height * scale;
    context.drawImage(work, bounds.x, bounds.y, bounds.width, bounds.height,
        (PROJECT_ICON_SIZE - drawWidth) / 2, (PROJECT_ICON_SIZE - drawHeight) / 2, drawWidth, drawHeight);
    return encode(canvas);
}

/** A decoded picture that can be cropped and rendered into an icon. */
export type IconImage = {image: CanvasImageSource; width: number; height: number};

/**
 * Square crop of an image. `zoom` ≥ 1 shrinks the square from the image's
 * shorter side; `x` and `y` (0–100) position it within the remaining space.
 */
export type IconCrop = {zoom: number; x: number; y: number};

export const DEFAULT_ICON_CROP: IconCrop = {zoom: 1, x: 50, y: 50};
export const MAX_ICON_ZOOM = 5;

async function decodeImage(url: string, failure: string): Promise<IconImage> {
    const image = new Image();
    image.src = url;
    try {
        await image.decode();
    } catch {
        throw new Error(failure);
    }
    return {image, width: image.naturalWidth || PROJECT_ICON_SIZE, height: image.naturalHeight || PROJECT_ICON_SIZE};
}

/** Validates and decodes a PNG, JPEG, GIF, WebP, or SVG file for cropping. */
export async function loadIconImage(file: File): Promise<IconImage> {
    const isSvgName = file.name.toLowerCase().endsWith('.svg');
    if (!ICON_FILE_TYPES.includes(file.type) && !isSvgName) throw new Error('Choose a PNG, JPEG, GIF, WebP, or SVG image.');
    if (file.size > MAX_ICON_FILE_BYTES) throw new Error('Choose an image smaller than 5 MB.');
    // Blob type matters for SVG files reported without a MIME type.
    const source = isSvgName && file.type !== 'image/svg+xml' ? new Blob([file], {type: 'image/svg+xml'}) : file;
    const url = URL.createObjectURL(source);
    try {
        return await decodeImage(url, 'The image could not be read. Choose a different file.');
    } finally {
        // A decoded image stays drawable after its object URL is released.
        URL.revokeObjectURL(url);
    }
}

/** Returns the source rectangle that `crop` selects from a width × height image. */
export function cropRect(width: number, height: number, crop: IconCrop): {x: number; y: number; size: number} {
    const zoom = Math.min(Math.max(crop.zoom, 1), MAX_ICON_ZOOM);
    const size = Math.min(width, height) / zoom;
    const position = (value: number) => Math.min(Math.max(value, 0), 100) / 100;
    return {x: (width - size) * position(crop.x), y: (height - size) * position(crop.y), size};
}

/**
 * Renders a decoded image as a square PNG data URL. Without `crop` the whole
 * image is fitted and letterboxed; with `crop` the selected square fills the icon.
 */
export function renderIconImage(source: IconImage, crop?: IconCrop): string {
    const [canvas, context] = iconCanvas();
    context.imageSmoothingQuality = 'high';
    if (crop) {
        const area = cropRect(source.width, source.height, crop);
        context.drawImage(source.image, area.x, area.y, area.size, area.size, 0, 0, PROJECT_ICON_SIZE, PROJECT_ICON_SIZE);
    } else {
        const scale = PROJECT_ICON_SIZE / Math.max(source.width, source.height);
        const drawWidth = source.width * scale, drawHeight = source.height * scale;
        context.drawImage(source.image, (PROJECT_ICON_SIZE - drawWidth) / 2, (PROJECT_ICON_SIZE - drawHeight) / 2, drawWidth, drawHeight);
    }
    return encode(canvas);
}

/** Scales a PNG, JPEG, GIF, WebP, or SVG file into a square PNG data URL. */
export async function imageFileIcon(file: File): Promise<string> {
    return renderIconImage(await loadIconImage(file));
}

/** Inverts the colors of a stored icon, keeping its transparency. */
export async function invertIcon(icon: string): Promise<string> {
    if (!icon.startsWith('data:image/png;base64,')) throw new Error('Only saved PNG icons can be inverted.');
    const source = await decodeImage(icon, 'The icon could not be read.');
    const [canvas, context] = iconCanvas();
    context.drawImage(source.image, 0, 0, PROJECT_ICON_SIZE, PROJECT_ICON_SIZE);
    const pixels = context.getImageData(0, 0, PROJECT_ICON_SIZE, PROJECT_ICON_SIZE);
    for (let i = 0; i < pixels.data.length; i += 4) {
        pixels.data[i] = 255 - pixels.data[i];
        pixels.data[i + 1] = 255 - pixels.data[i + 1];
        pixels.data[i + 2] = 255 - pixels.data[i + 2];
    }
    context.putImageData(pixels, 0, 0);
    return encode(canvas);
}
