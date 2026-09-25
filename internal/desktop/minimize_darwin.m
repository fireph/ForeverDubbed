//go:build gui && darwin && !ci

#import <AppKit/AppKit.h>
#import <pthread.h>
#include <stdint.h>

int fdb_gui_minimized(uintptr_t window, int restore) {
    __block int result = 0;
    void (^check)(void) = ^{
      NSWindow *w = (__bridge NSWindow *)(void *)window;
      result = [w isMiniaturized];
      if (restore && result)
          [w deminiaturize:nil];
    };
    if (pthread_main_np())
        check();
    else
        dispatch_sync(dispatch_get_main_queue(), check);
    return result;
}
