import {afterEach, describe, expect, it, vi} from 'vitest';
import {cropRect, emojiIcon, imageFileIcon, invertIcon, MAX_ICON_FILE_BYTES, MAX_PROJECT_ICON_LENGTH, opaqueBounds, PROJECT_ICON_SIZE, renderIconImage} from './projectIcon';

const png = 'data:image/png;base64,iVBORw0KGgo=';

function fakeContext() {
    return {
        font: '', fillStyle: '', textAlign: '', textBaseline: '', imageSmoothingQuality: '',
        measureText: vi.fn((text: string) => ({width: text.length * 40})),
        fillText: vi.fn(),
        drawImage: vi.fn(),
        getImageData: vi.fn(() => ({data: Uint8ClampedArray.from([0, 10, 255, 128, 200, 100, 50, 0])})),
        putImageData: vi.fn(),
    };
}

function mockCanvas(dataUrl = png) {
    const context = fakeContext();
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(context as unknown as CanvasRenderingContext2D);
    vi.spyOn(HTMLCanvasElement.prototype, 'toDataURL').mockReturnValue(dataUrl);
    return context;
}

afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
});

function glyphPixels(box: {x: number; y: number; width: number; height: number} | null) {
    const data = new Uint8ClampedArray(256 * 256 * 4);
    if (box) {
        for (let y = box.y; y < box.y + box.height; y++) {
            for (let x = box.x; x < box.x + box.width; x++) data[(y * 256 + x) * 4 + 3] = 255;
        }
    }
    return {data};
}

describe('opaqueBounds', () => {
    it.each([
        ['all transparent', [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0], 2, 2, null],
        ['single pixel', [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1], 2, 2, {x: 1, y: 1, width: 1, height: 1}],
        ['faint and opaque pixels', [0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255], 2, 2, {x: 0, y: 0, width: 2, height: 2}],
    ])('%s', (_name, data, width, height, expected) => {
        expect(opaqueBounds(data, width, height)).toEqual(expected);
    });
});

describe('emojiIcon', () => {
    it('draws trimmed text in the project color', () => {
        const context = mockCanvas();
        context.getImageData.mockReturnValue(glyphPixels({x: 100, y: 100, width: 20, height: 20}));
        expect(emojiIcon('  🚀 ', '#ff0000')).toBe(png);
        expect(context.fillStyle).toBe('#ff0000');
        expect(context.fillText).toHaveBeenCalledWith('🚀', 128, 128);
    });

    it.each([
        ['square glyph fills the icon', {x: 40, y: 50, width: 100, height: 100}, [40, 50, 100, 100, 0, 0, 64, 64]],
        ['wide text keeps aspect ratio', {x: 10, y: 100, width: 200, height: 50}, [10, 100, 200, 50, 0, 24, 64, 16]],
        ['tall glyph keeps aspect ratio', {x: 100, y: 0, width: 32, height: 128}, [100, 0, 32, 128, 24, 0, 16, 64]],
    ])('trims to visible pixels: %s', (_name, box, expected) => {
        const context = mockCanvas();
        context.getImageData.mockReturnValue(glyphPixels(box));
        emojiIcon('🚀', '#000000');
        expect(context.drawImage).toHaveBeenCalledWith(expect.anything(), ...expected);
    });

    it('shrinks long text to fit the working canvas', () => {
        const context = mockCanvas();
        context.getImageData.mockReturnValue(glyphPixels({x: 0, y: 0, width: 10, height: 10}));
        emojiIcon('ABCDEFGHIJKL', '#000000');
        expect(context.font).toMatch(/^16px /);
    });

    it('rejects text without visible pixels', () => {
        const context = mockCanvas();
        context.getImageData.mockReturnValue(glyphPixels(null));
        expect(() => emojiIcon('\u200b', '#000000')).toThrow('no visible characters');
    });

    it.each([
        ['empty text', '   ', png, 'Enter an emoji or short text first.'],
        ['non-PNG output', 'A', 'data:,', 'could not be converted to PNG'],
        ['oversized output', 'A', `${png}${'A'.repeat(MAX_PROJECT_ICON_LENGTH)}`, 'too detailed'],
    ])('rejects %s', (_name, text, output, message) => {
        const context = mockCanvas(output);
        context.getImageData.mockReturnValue(glyphPixels({x: 0, y: 0, width: 10, height: 10}));
        expect(() => emojiIcon(text, '#000000')).toThrow(message);
    });

    it('reports a missing canvas context', () => {
        vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null);
        expect(() => emojiIcon('A', '#000000')).toThrow('Icons cannot be drawn');
    });
});

describe('imageFileIcon', () => {
    function stubImage(width: number, height: number, decode: () => Promise<void> = () => Promise.resolve()) {
        const images: {src: string}[] = [];
        vi.stubGlobal('Image', class {
            src = '';
            naturalWidth = width;
            naturalHeight = height;
            decode = decode;
            constructor() { images.push(this); }
        });
        vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:icon');
        const revoke = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined);
        return {images, revoke};
    }

    it('fits wide images into the square icon and releases the object URL', async () => {
        const context = mockCanvas();
        const {revoke} = stubImage(200, 100);
        await expect(imageFileIcon(new File(['x'], 'logo.png', {type: 'image/png'}))).resolves.toBe(png);
        expect(context.drawImage).toHaveBeenCalledWith(expect.anything(), 0, 16, 64, 32);
        expect(revoke).toHaveBeenCalledWith('blob:icon');
    });

    it('accepts SVG files without a MIME type and sizes them to the icon', async () => {
        const context = mockCanvas();
        stubImage(0, 0);
        await imageFileIcon(new File(['<svg/>'], 'logo.SVG'));
        const blob = vi.mocked(URL.createObjectURL).mock.calls[0][0];
        expect(blob).toBeInstanceOf(Blob);
        expect((blob as Blob).type).toBe('image/svg+xml');
        expect(context.drawImage).toHaveBeenCalledWith(expect.anything(), 0, 0, 64, 64);
    });

    it.each([
        ['unsupported type', new File(['x'], 'notes.txt', {type: 'text/plain'}), 'Choose a PNG, JPEG, GIF, WebP, or SVG image.'],
        ['oversized file', new File([new Uint8Array(MAX_ICON_FILE_BYTES + 1)], 'big.png', {type: 'image/png'}), 'smaller than 5 MB'],
    ])('rejects an %s', async (_name, file, message) => {
        await expect(imageFileIcon(file)).rejects.toThrow(message);
    });

    it('reports unreadable images and still releases the object URL', async () => {
        mockCanvas();
        const {revoke} = stubImage(10, 10, () => Promise.reject(new Error('decode failed')));
        await expect(imageFileIcon(new File(['x'], 'broken.png', {type: 'image/png'}))).rejects.toThrow('could not be read');
        expect(revoke).toHaveBeenCalledWith('blob:icon');
    });
});

describe('cropRect', () => {
    it.each([
        ['default crop centers the largest square of a wide image', 200, 100, {zoom: 1, x: 50, y: 50}, {x: 50, y: 0, size: 100}],
        ['left edge', 200, 100, {zoom: 1, x: 0, y: 50}, {x: 0, y: 0, size: 100}],
        ['zoom shrinks the square', 200, 100, {zoom: 2, x: 100, y: 100}, {x: 150, y: 50, size: 50}],
        ['tall image moves vertically', 100, 300, {zoom: 1, x: 50, y: 25}, {x: 0, y: 50, size: 100}],
        ['zoom and position are clamped', 100, 100, {zoom: 99, x: -10, y: 200}, {x: 0, y: 80, size: 20}],
        ['zoom below 1 is clamped', 100, 100, {zoom: 0.5, x: 50, y: 50}, {x: 0, y: 0, size: 100}],
    ])('%s', (_name, width, height, crop, expected) => {
        expect(cropRect(width, height, crop)).toEqual(expected);
    });
});

describe('renderIconImage', () => {
    const source = {image: document.createElement('canvas'), width: 200, height: 100};

    it('fits the whole image without a crop', () => {
        const context = mockCanvas();
        expect(renderIconImage(source)).toBe(png);
        expect(context.drawImage).toHaveBeenCalledWith(source.image, 0, 16, 64, 32);
    });

    it('fills the icon with the cropped square', () => {
        const context = mockCanvas();
        expect(renderIconImage(source, {zoom: 2, x: 0, y: 100})).toBe(png);
        expect(context.drawImage).toHaveBeenCalledWith(source.image, 0, 50, 50, 50, 0, 0, PROJECT_ICON_SIZE, PROJECT_ICON_SIZE);
    });
});

describe('invertIcon', () => {
    function stubIconImage(decode: () => Promise<void> = () => Promise.resolve()) {
        vi.stubGlobal('Image', class {
            src = '';
            naturalWidth = 64;
            naturalHeight = 64;
            decode = decode;
        });
    }

    it('inverts colors and keeps transparency', async () => {
        const context = mockCanvas();
        stubIconImage();
        await expect(invertIcon(png)).resolves.toBe(png);
        expect(context.drawImage).toHaveBeenCalledWith(expect.anything(), 0, 0, PROJECT_ICON_SIZE, PROJECT_ICON_SIZE);
        const [pixels, x, y] = context.putImageData.mock.calls[0] as unknown as [{data: Uint8ClampedArray}, number, number];
        expect([...pixels.data]).toEqual([255, 245, 0, 128, 55, 155, 205, 0]);
        expect([x, y]).toEqual([0, 0]);
    });

    it.each([
        ['non-PNG icons', 'data:image/svg+xml;base64,PHN2Zy8+', () => Promise.resolve(), 'Only saved PNG icons'],
        ['unreadable icons', png, () => Promise.reject(new Error('bad')), 'could not be read'],
    ])('rejects %s', async (_name, icon, decode, message) => {
        mockCanvas();
        stubIconImage(decode);
        await expect(invertIcon(icon)).rejects.toThrow(message);
    });
});
