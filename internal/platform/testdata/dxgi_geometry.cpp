#include "dxgi_geometry.h"
#include <cassert>
#include <vector>

int main() {
    // Only supported Windows 10 client builds use desktop capture.
    assert(fdb::useDesktopDuplication(10, 0, 18362, 1));
    assert(fdb::useDesktopDuplication(10, 0, 19045, 1));
    assert(!fdb::useDesktopDuplication(10, 0, 17763, 1));
    assert(!fdb::useDesktopDuplication(10, 0, 22000, 1));
    assert(!fdb::useDesktopDuplication(10, 0, 26100, 1));
    assert(!fdb::useDesktopDuplication(10, 0, 20348, 3)); // Server
    assert(!fdb::useDesktopDuplication(10, 0, 19045, 2)); // Domain controller
    assert(!fdb::useDesktopDuplication(6, 3, 9600, 1));
    assert(!fdb::useDesktopDuplication(11, 0, 30000, 1));

    // A 3x2 texture holding pixels 1,2,3 / 4,5,6. Expected visible images
    // are specified independently for each physical monitor orientation.
    const unsigned char expected[][6] = {
        {1, 2, 3, 4, 5, 6}, {1, 2, 3, 4, 5, 6},
        {4, 1, 5, 2, 6, 3}, {6, 5, 4, 3, 2, 1}, {3, 6, 2, 5, 1, 4}
    };
    for (unsigned rotation = 0; rotation <= 4; ++rotation) {
        const bool portrait = rotation == 2 || rotation == 4;
        const int w = portrait ? 2 : 3, h = portrait ? 3 : 2;
        // Negative monitor origins and every possible non-empty crop, including
        // requests crossing monitor edges. This also exercises snapshot assembly.
        const fdb::Rect monitor{-20, -10, -20 + w, -10 + h};
        for (int left = -1; left < w; ++left)
        for (int top = -1; top < h; ++top)
        for (int right = left + 1; right <= w + 1; ++right)
        for (int bottom = top + 1; bottom <= h + 1; ++bottom) {
            fdb::Rect request{monitor.left + left, monitor.top + top,
                              monitor.left + right, monitor.top + bottom};
            auto overlap = fdb::intersect(request, monitor);
            if (overlap.empty()) continue;
            fdb::Rect local{overlap.left - monitor.left, overlap.top - monitor.top,
                            overlap.right - monitor.left, overlap.bottom - monitor.top};
            auto source = fdb::textureRect(local, 3, 2, rotation);
            assert(source.left >= 0 && source.top >= 0 && source.right <= 3 && source.bottom <= 2);
            size_t pitch = source.width() * 4 + 12; // GPU row pitch includes padding.
            std::vector<unsigned char> bgra(pitch * source.height(), 0xee);
            for (int y = source.top; y < source.bottom; ++y)
            for (int x = source.left; x < source.right; ++x) {
                auto pixel = bgra.data() + (y - source.top) * pitch + (x - source.left) * 4;
                pixel[0] = 101; pixel[1] = 51; pixel[2] = y * 3 + x + 1; pixel[3] = 0;
            }
            std::vector<unsigned char> rgba(request.width() * request.height() * 4, 0);
            fdb::copyDesktopCrop(rgba.data(), request, overlap, monitor, bgra.data(), pitch,
                                 source, 3, 2, rotation);
            for (int y = top; y < bottom; ++y)
            for (int x = left; x < right; ++x) {
                auto pixel = rgba.data() + ((y - top) * request.width() + x - left) * 4;
                if (x >= 0 && x < w && y >= 0 && y < h) {
                    assert(pixel[0] == expected[rotation][y * w + x]);
                    assert(pixel[1] == 51 && pixel[2] == 101 && pixel[3] == 255);
                } else {
                    assert(pixel[0] == 0 && pixel[1] == 0 && pixel[2] == 0 && pixel[3] == 0);
                }
            }
        }
    }
    // A game straddling two outputs assembles both intersections into one crop.
    fdb::Rect request{-1, 0, 1, 1}, first{-3, 0, 0, 2}, second{0, 0, 3, 2};
    unsigned char leftPixel[]{30, 20, 10, 0}, rightPixel[]{60, 50, 40, 0}, rgba[8]{};
    fdb::copyDesktopCrop(rgba, request, {-1, 0, 0, 1}, first, leftPixel, 4, {2, 0, 3, 1}, 3, 2, 1);
    fdb::copyDesktopCrop(rgba, request, {0, 0, 1, 1}, second, rightPixel, 4, {0, 0, 1, 1}, 3, 2, 1);
    assert(rgba[0] == 10 && rgba[2] == 30 && rgba[3] == 255);
    assert(rgba[4] == 40 && rgba[6] == 60 && rgba[7] == 255);
}
