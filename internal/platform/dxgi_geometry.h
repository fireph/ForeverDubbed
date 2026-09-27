#ifndef FDB_DXGI_GEOMETRY_H
#define FDB_DXGI_GEOMETRY_H
#include <algorithm>
#include <cstddef>

namespace fdb {
// Windows 11 also reports major version 10. Exclude Server and unknown/new OSes.
inline bool useDesktopDuplication(unsigned major, unsigned minor, unsigned build,
                                  unsigned productType) {
    return major == 10 && minor == 0 && build >= 18362 && build < 22000 && productType == 1;
}
struct Point { int x, y; };
struct Rect {
    int left, top, right, bottom;
    int width() const { return right - left; }
    int height() const { return bottom - top; }
    bool empty() const { return right <= left || bottom <= top; }
};
inline Rect intersect(Rect a, Rect b) {
    return {std::max(a.left, b.left), std::max(a.top, b.top),
            std::min(a.right, b.right), std::min(a.bottom, b.bottom)};
}
// DXGI rotation values: 0 unspecified, 1 identity, 2/3/4 = 90/180/270 clockwise.
// Convert displayed pixels to coordinates in the unrotated duplication texture.
inline Point texturePoint(int x, int y, int width, int height, unsigned rotation) {
    switch (rotation) {
    case 2: return {y, height - 1 - x};
    case 3: return {width - 1 - x, height - 1 - y};
    case 4: return {width - 1 - y, x};
    default: return {x, y};
    }
}
inline Rect textureRect(Rect displayed, int width, int height, unsigned rotation) {
    Point a = texturePoint(displayed.left, displayed.top, width, height, rotation);
    Point b = texturePoint(displayed.right - 1, displayed.bottom - 1, width, height, rotation);
    return {std::min(a.x, b.x), std::min(a.y, b.y),
            std::max(a.x, b.x) + 1, std::max(a.y, b.y) + 1};
}
// Copy only the requested desktop intersection, honoring GPU row padding and rotation.
inline void copyDesktopCrop(unsigned char *rgba, Rect request, Rect overlap, Rect monitor,
                            const unsigned char *bgra, size_t pitch, Rect source,
                            int textureWidth, int textureHeight, unsigned rotation) {
    for (int y = overlap.top; y < overlap.bottom; ++y) {
        auto dest = rgba + (size_t(y - request.top) * request.width() + overlap.left - request.left) * 4;
        for (int x = overlap.left; x < overlap.right; ++x) {
            Point p = texturePoint(x - monitor.left, y - monitor.top, textureWidth, textureHeight, rotation);
            auto pixel = bgra + size_t(p.y - source.top) * pitch + size_t(p.x - source.left) * 4;
            *dest++ = pixel[2];
            *dest++ = pixel[1];
            *dest++ = pixel[0];
            *dest++ = 255;
        }
    }
}
}
#endif
