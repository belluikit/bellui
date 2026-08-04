//go:build ios

#import <Foundation/Foundation.h>

#import <stdbool.h>

void iosDeletePath(const char* path) {
    NSString *pathStr = [NSString stringWithUTF8String:path];
    [[NSFileManager defaultManager] removeItemAtPath:pathStr error:nil];
}

bool iosExistsPath(const char* path) {
    NSString *pathStr = [NSString stringWithUTF8String:path];
    return [[NSFileManager defaultManager] fileExistsAtPath:pathStr];
}

// This file is compiled without ARC, so every object handed to Go as a raw
// pointer must be explicitly retained here and released via iosReleaseObject
// (or the close functions below) — an autoreleased pointer dies with the next
// pool drain and the later use from Go is a use-after-free (#250).
void* iosParseUrl(const char* url) {
    NSString *urlStr = [NSString stringWithUTF8String:url];
    NSURL *parsed = [NSURL URLWithString:urlStr];
    if (parsed != nil) {
        CFRetain((CFTypeRef)parsed);
    }
    return parsed;
}

void iosReleaseObject(void* ptr) {
    if (ptr != NULL) {
        CFRelease((CFTypeRef)ptr);
    }
}

const void* iosReadFromURL(void* urlPtr, int* len)  {
    NSURL* url = (NSURL*)urlPtr;
    NSData* data = [NSData dataWithContentsOfURL:url];

    *len = data.length;
    return data.bytes;
}

const void* iosOpenFileWriter(void* urlPtr, bool truncate) {
    NSURL* url = (NSURL*)urlPtr;

    if (truncate || ![[NSFileManager defaultManager] fileExistsAtPath:url.path]) {
        [[NSFileManager defaultManager] createFileAtPath:url.path contents:nil attributes:nil];
    }

    NSError *err = nil;
    // TODO handle error
    NSFileHandle* handle = [NSFileHandle fileHandleForWritingToURL:url error:&err];
    if (handle == nil) {
        return NULL;
    }

    if (!truncate) {
        [handle seekToEndOfFile];
    }

    CFRetain((CFTypeRef)handle);
    return handle;
}

void iosCloseFileWriter(void* handlePtr) {
    if (handlePtr == NULL) {
        return;
    }
    NSFileHandle* handle = (NSFileHandle*)handlePtr;

    [handle synchronizeFile];
    [handle closeFile];
    CFRelease((CFTypeRef)handlePtr);
}


const int iosWriteToFile(void* handlePtr, const void* bytes, int len) {
    NSFileHandle* handle = (NSFileHandle*)handlePtr;
    NSData *data = [NSData dataWithBytes:bytes length:len];

    [handle writeData:data];
    return data.length;
}
