// The probe's audio: a playback AVAudioSession and an AVAudioEngine source
// node that renders a quiet 440 Hz beep, counting every frame it renders.

#import <AVFoundation/AVFoundation.h>
#import <UIKit/UIKit.h>
#include <math.h>
#include <stdatomic.h>
#include <stdlib.h>
#include <string.h>

static AVAudioEngine *engine;
static _Atomic uint64_t framesRendered;
static _Atomic int appState; // 0 active, 1 inactive, 2 background (UIApplicationState)
static _Atomic int interruptions;

static const char *dupErr(NSString *what, NSError *err) {
	NSString *s = [NSString stringWithFormat:@"%@: %@", what, err ? err.localizedDescription : @"failed"];
	return strdup(s.UTF8String);
}

static void observeAppState(void) {
	NSNotificationCenter *nc = [NSNotificationCenter defaultCenter];
	NSDictionary<NSNotificationName, NSNumber *> *states = @{
		UIApplicationDidBecomeActiveNotification : @(0),
		UIApplicationWillResignActiveNotification : @(1),
		UIApplicationDidEnterBackgroundNotification : @(2),
		UIApplicationWillEnterForegroundNotification : @(1),
	};
	for (NSNotificationName name in states) {
		int v = states[name].intValue;
		[nc addObserverForName:name object:nil queue:nil usingBlock:^(NSNotification *n) {
			atomic_store(&appState, v);
		}];
	}
	[nc addObserverForName:AVAudioSessionInterruptionNotification object:nil queue:nil usingBlock:^(NSNotification *n) {
		atomic_fetch_add(&interruptions, 1);
	}];
}

static const char *startOnMain(void) {
	__block const char *result = NULL;
	void (^work)(void) = ^{
		atomic_store(&appState, (int)[UIApplication sharedApplication].applicationState);
		observeAppState();

		NSError *err = nil;
		AVAudioSession *session = [AVAudioSession sharedInstance];
		if (![session setCategory:AVAudioSessionCategoryPlayback error:&err]) {
			result = dupErr(@"setCategory", err);
			return;
		}
		if (![session setActive:YES error:&err]) {
			result = dupErr(@"setActive", err);
			return;
		}

		engine = [[AVAudioEngine alloc] init];
		AVAudioFormat *out = [engine.outputNode inputFormatForBus:0];
		double rate = out.sampleRate > 0 ? out.sampleRate : 48000;
		AVAudioFormat *fmt = [[AVAudioFormat alloc] initStandardFormatWithSampleRate:rate channels:1];
		__block double phase = 0;
		__block uint64_t pos = 0;
		AVAudioSourceNode *src = [[AVAudioSourceNode alloc] initWithFormat:fmt
			renderBlock:^OSStatus(BOOL *silence, const AudioTimeStamp *ts, AVAudioFrameCount n, AudioBufferList *abl) {
				float *buf = (float *)abl->mBuffers[0].mData;
				for (AVAudioFrameCount i = 0; i < n; i++, pos++) {
					BOOL on = (pos / (uint64_t)(rate / 2)) % 2 == 0; // 0.5 s on, 0.5 s off
					buf[i] = on ? 0.08f * (float)sin(phase) : 0;
					phase += 2 * M_PI * 440 / rate;
					if (phase > 2 * M_PI) phase -= 2 * M_PI;
				}
				atomic_fetch_add(&framesRendered, n);
				return noErr;
			}];
		[engine attachNode:src];
		[engine connect:src to:engine.mainMixerNode format:fmt];
		if (![engine startAndReturnError:&err]) {
			result = dupErr(@"engine start", err);
		}
	};
	if ([NSThread isMainThread]) {
		work();
	} else {
		dispatch_sync(dispatch_get_main_queue(), work);
	}
	return result;
}

const char *bgaStart(void) { return startOnMain(); }

uint64_t bgaFrames(void) { return atomic_load(&framesRendered); }
int bgaRunning(void) { return engine.isRunning ? 1 : 0; }
int bgaState(void) { return atomic_load(&appState); }
int bgaInterruptions(void) { return atomic_load(&interruptions); }

void bgaStop(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[engine stop];
		[[AVAudioSession sharedInstance] setActive:NO error:nil];
	});
}
