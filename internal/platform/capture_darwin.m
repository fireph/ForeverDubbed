//go:build darwin && cgo

#import <Foundation/Foundation.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
#import <CoreGraphics/CoreGraphics.h>
#include <math.h>
#include <libproc.h>
#include "native_darwin.h"

// This file is compiled with ARC; callbacks own their results even if a wait
// times out. They never retain Go memory or stack pointers.
static NSArray<SCWindow *> *availableWindows;
static SCContentFilter *windowFilter;
static int fail(char *error, size_t size, NSString *message) {
    snprintf(error, size, "%s", message.UTF8String);
    return -1;
}

int fdb_screen_init(char *error, size_t size) {
    @autoreleasepool {
        // A command-line process has no AppKit startup to initialize the
        // WindowServer connection. The independent-window filter can otherwise
        // abort in CGS_REQUIRE_INIT, even after permission/discovery succeeds.
        // Reading a display ID initializes CoreGraphics; it captures no pixels.
        if (CGMainDisplayID() == kCGNullDirectDisplay)
            return fail(error, size,
                        @"No macOS desktop session is available; run from a terminal in your "
                        @"logged-in desktop session.");
        if (!CGPreflightScreenCaptureAccess()) {
            CGRequestScreenCaptureAccess();
            return fail(error, size,
                        @"Allow Screen Recording for your terminal or ForeverDubbed in System "
                        @"Settings > Privacy & Security, then quit and reopen it.");
        }
        return 0;
    }
}

int fdb_window_list(char **json, char *error, size_t size) {
    @autoreleasepool {
        // Read metadata only. No pixels from these windows/displays are captured.
        availableWindows = nil;
        windowFilter = nil;
        *json = NULL;
        dispatch_semaphore_t done = dispatch_semaphore_create(0);
        __block SCShareableContent *content = nil;
        __block NSError *failure = nil;
        [SCShareableContent
            getShareableContentExcludingDesktopWindows:YES
                                   onScreenWindowsOnly:YES
                                     completionHandler:^(SCShareableContent *result, NSError *err) {
                                       content = result;
                                       failure = err;
                                       dispatch_semaphore_signal(done);
                                     }];
        if (dispatch_semaphore_wait(done, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC)))
            return fail(error, size,
                        @"Timed out listing game windows; check Screen Recording permission.");
        if (failure)
            return fail(error, size, failure.localizedDescription);
        availableWindows = content.windows;
        NSMutableArray *metadata = [NSMutableArray array];
        for (SCWindow *window in availableWindows) {
            SCRunningApplication *app = window.owningApplication;
            if (!app)
                continue;
            char executable[PROC_PIDPATHINFO_MAXSIZE] = {0};
            NSString *executablePath = @"";
            if (proc_pidpath(app.processID, executable, sizeof(executable)) > 0)
                executablePath = [NSString stringWithUTF8String:executable] ?: @"";
            [metadata addObject:@{
                @"ID" : @(window.windowID),
                @"PID" : @(app.processID),
                @"App" : app.applicationName ?: @"",
                @"Bundle" : app.bundleIdentifier ?: @"",
                @"Executable" : executablePath,
                @"Width" : @(window.frame.size.width),
                @"Height" : @(window.frame.size.height),
                @"Layer" : @(window.windowLayer),
                @"OnScreen" : @(window.isOnScreen)
            }];
        }
        NSError *jsonError = nil;
        NSData *data = [NSJSONSerialization dataWithJSONObject:metadata options:0 error:&jsonError];
        if (!data)
            return fail(error, size, jsonError.localizedDescription);
        *json = calloc(data.length + 1, 1);
        if (!*json)
            return fail(error, size, @"Could not allocate window metadata.");
        memcpy(*json, data.bytes, data.length);
        return 0;
    }
}

int fdb_window_select(uint32_t window_id, int32_t pid, int *width, int *height, char *error,
                      size_t size) {
    @autoreleasepool {
        windowFilter = nil;
        for (SCWindow *window in availableWindows) {
            if (window.windowID != window_id || window.owningApplication.processID != pid)
                continue;
            // This is the only content filter used by the macOS backend. It
            // cannot capture the desktop or pixels belonging to other apps.
            windowFilter = [[SCContentFilter alloc] initWithDesktopIndependentWindow:window];
            *width =
                (int)llround(windowFilter.contentRect.size.width * windowFilter.pointPixelScale);
            *height =
                (int)llround(windowFilter.contentRect.size.height * windowFilter.pointPixelScale);
            return 0;
        }
        return fail(error, size, @"Game window closed; waiting for it to reopen.");
    }
}

int fdb_window_capture(int x, int y, int width, int height, void *pixels, char *error,
                       size_t size) {
    @autoreleasepool {
        SCContentFilter *filter = windowFilter;
        if (!filter)
            return fail(error, size, @"No game window selected.");
        int fullWidth = (int)llround(filter.contentRect.size.width * filter.pointPixelScale);
        int fullHeight = (int)llround(filter.contentRect.size.height * filter.pointPixelScale);
        if (x < 0 || y < 0 || width <= 0 || height <= 0 || (int64_t)x + width > fullWidth ||
            (int64_t)y + height > fullHeight)
            return fail(error, size, @"Capture rectangle is outside the game window.");
        SCStreamConfiguration *config = [SCStreamConfiguration new];
        // ScreenCaptureKit ignores sourceRect for independent windows. Capture
        // the selected window at native size, then crop the tile from its image.
        config.width = fullWidth;
        config.height = fullHeight;
        config.ignoreShadowsSingleWindow = YES;
        config.showsCursor = NO;
        config.scalesToFit = NO;
        config.colorSpaceName = kCGColorSpaceSRGB;
        dispatch_semaphore_t done = dispatch_semaphore_create(0);
        __block NSData *data = nil;
        __block NSString *failure = nil;
        [SCScreenshotManager
            captureImageWithFilter:filter
                     configuration:config
                 completionHandler:^(CGImageRef image, NSError *err) {
                   if (err || !image) {
                       failure = err.localizedDescription ?: @"ScreenCaptureKit returned no image.";
                   } else if (CGImageGetWidth(image) != fullWidth ||
                              CGImageGetHeight(image) != fullHeight) {
                       failure = @"Game window size changed; searching for it again.";
                   } else {
                       CGImageRef cropped =
                           CGImageCreateWithImageInRect(image, CGRectMake(x, y, width, height));
                       NSMutableData *rgba =
                           [NSMutableData dataWithLength:(size_t)width * height * 4];
                       CGColorSpaceRef colors = CGColorSpaceCreateWithName(kCGColorSpaceSRGB);
                       CGContextRef context = CGBitmapContextCreate(
                           rgba.mutableBytes, width, height, 8, (size_t)width * 4, colors,
                           kCGImageAlphaPremultipliedLast | kCGBitmapByteOrder32Big);
                       CGColorSpaceRelease(colors);
                       if (context && cropped) {
                           // Drawing a CGImage into an untransformed bitmap preserves
                           // its top-to-bottom memory rows. A UIKit-style Y flip here
                           // would mirror the tile and break protocol decoding.
                           CGContextSetInterpolationQuality(context, kCGInterpolationNone);
                           CGContextDrawImage(context, CGRectMake(0, 0, width, height), cropped);
                           data = rgba;
                       } else {
                           failure = @"Could not allocate game window bitmap.";
                       }
                       if (cropped)
                           CGImageRelease(cropped);
                       if (context)
                           CGContextRelease(context);
                   }
                   dispatch_semaphore_signal(done);
                 }];
        if (dispatch_semaphore_wait(done, dispatch_time(DISPATCH_TIME_NOW, 5 * NSEC_PER_SEC)))
            return fail(error, size,
                        @"Screen capture timed out; check Screen Recording permission.");
        if (!data)
            return fail(error, size, failure ?: @"Screen capture failed.");
        memcpy(pixels, data.bytes, data.length);
        return 0;
    }
}
