//go:build ios

#include <stdlib.h>
#include <string.h>
#import <AVFoundation/AVFoundation.h>

// mmediaSetPlaybackSession sets the shared audio session's category to
// Playback and activates it. It returns NULL, or a malloc'd description of
// the error, which the caller frees.
char *mmediaSetPlaybackSession(void) {
	NSError *err = nil;
	AVAudioSession *s = [AVAudioSession sharedInstance];
	if (![s setCategory:AVAudioSessionCategoryPlayback error:&err]) {
		return strdup([[err description] UTF8String]);
	}
	if (![s setActive:YES error:&err]) {
		return strdup([[err description] UTF8String]);
	}
	return NULL;
}
