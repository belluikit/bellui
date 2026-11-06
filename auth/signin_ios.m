//go:build ios

// The native half of sign-in on iOS: the Sign in with Apple sheet and
// ASWebAuthenticationSession. Each request runs on the UIKit main thread and
// reports back through the Go exports with the token it started with. The
// objects a request needs alive until it completes are held in `live`, keyed
// by that token.

#import <AuthenticationServices/AuthenticationServices.h>
#import <UIKit/UIKit.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#include "_cgo_export.h"

extern UIWindow *belluiAppWindow(void);

// Status values, matching nativeOK/nativeCancelled/nativeFailed in signin_ios.go.
enum { authOK = 0, authCancelled = 1, authFailed = 2 };

static NSMutableDictionary<NSNumber *, id> *live;

static void keep(uint64_t token, id obj) {
    if (live == nil) {
        live = [NSMutableDictionary dictionary];
    }
    live[@(token)] = obj;
}

static void drop(uint64_t token) { [live removeObjectForKey:@(token)]; }

static char *cstr(NSString *s) { return (char *)(s ? s.UTF8String : ""); }

static ASPresentationAnchor anchor(void) {
    UIWindow *w = belluiAppWindow();
    if (w != nil) {
        return w;
    }
    for (UIScene *scene in UIApplication.sharedApplication.connectedScenes) {
        if ([scene isKindOfClass:[UIWindowScene class]]) {
            for (UIWindow *win in ((UIWindowScene *)scene).windows) {
                if (win.isKeyWindow) {
                    return win;
                }
            }
        }
    }
    return nil;
}

@interface BelluiAppleSignIn
    : NSObject <ASAuthorizationControllerDelegate, ASAuthorizationControllerPresentationContextProviding>
@property(nonatomic) uint64_t token;
@property(nonatomic, strong) ASAuthorizationController *controller;
@end

@implementation BelluiAppleSignIn

- (void)authorizationController:(ASAuthorizationController *)controller
    didCompleteWithAuthorization:(ASAuthorization *)authorization {
    uint64_t token = self.token;
    if (![authorization.credential isKindOfClass:[ASAuthorizationAppleIDCredential class]]) {
        belluiAuthAppleDone(token, authFailed, "", "", "", "", "the sheet returned no Apple ID credential");
        drop(token);
        return;
    }
    ASAuthorizationAppleIDCredential *c = (ASAuthorizationAppleIDCredential *)authorization.credential;
    NSString *idToken = c.identityToken ? [[NSString alloc] initWithData:c.identityToken
                                                                 encoding:NSUTF8StringEncoding]
                                        : nil;
    NSString *code = c.authorizationCode ? [[NSString alloc] initWithData:c.authorizationCode
                                                                  encoding:NSUTF8StringEncoding]
                                         : nil;
    NSString *name = nil;
    if (c.fullName != nil) {
        name = [NSPersonNameComponentsFormatter localizedStringFromPersonNameComponents:c.fullName
                                                                                  style:NSPersonNameComponentsFormatterStyleDefault
                                                                                options:0];
    }
    if (idToken.length == 0) {
        belluiAuthAppleDone(token, authFailed, "", "", "", "", "the sheet returned no identity token");
    } else {
        belluiAuthAppleDone(token, authOK, cstr(idToken), cstr(code), cstr(c.email), cstr(name), "");
    }
    drop(token);
}

- (void)authorizationController:(ASAuthorizationController *)controller
           didCompleteWithError:(NSError *)error {
    uint64_t token = self.token;
    if ([error.domain isEqualToString:ASAuthorizationErrorDomain] && error.code == ASAuthorizationErrorCanceled) {
        belluiAuthAppleDone(token, authCancelled, "", "", "", "", "");
    } else {
        NSString *msg = [NSString stringWithFormat:@"%@ (%@ %ld)", error.localizedDescription, error.domain,
                                                   (long)error.code];
        belluiAuthAppleDone(token, authFailed, "", "", "", "", cstr(msg));
    }
    drop(token);
}

- (ASPresentationAnchor)presentationAnchorForAuthorizationController:(ASAuthorizationController *)controller {
    return anchor();
}

@end

@interface BelluiWebSession : NSObject <ASWebAuthenticationPresentationContextProviding>
@property(nonatomic, strong) ASWebAuthenticationSession *session;
@end

@implementation BelluiWebSession

- (ASPresentationAnchor)presentationAnchorForWebAuthenticationSession:(ASWebAuthenticationSession *)session {
    return anchor();
}

@end

void belluiAuthAppleSignIn(uint64_t token, const char *nonce) {
    NSString *n = [NSString stringWithUTF8String:nonce];
    dispatch_async(dispatch_get_main_queue(), ^{
      ASAuthorizationAppleIDRequest *req = [[[ASAuthorizationAppleIDProvider alloc] init] createRequest];
      req.requestedScopes = @[ ASAuthorizationScopeFullName, ASAuthorizationScopeEmail ];
      req.nonce = n;

      BelluiAppleSignIn *d = [[BelluiAppleSignIn alloc] init];
      d.token = token;
      d.controller = [[ASAuthorizationController alloc] initWithAuthorizationRequests:@[ req ]];
      d.controller.delegate = d;
      d.controller.presentationContextProvider = d;
      keep(token, d);
      [d.controller performRequests];
    });
}

void belluiAuthWebSession(uint64_t token, const char *url, const char *scheme) {
    NSURL *u = [NSURL URLWithString:[NSString stringWithUTF8String:url]];
    NSString *s = [NSString stringWithUTF8String:scheme];
    dispatch_async(dispatch_get_main_queue(), ^{
      BelluiWebSession *w = [[BelluiWebSession alloc] init];
      w.session = [[ASWebAuthenticationSession alloc]
                initWithURL:u
          callbackURLScheme:s
          completionHandler:^(NSURL *callbackURL, NSError *error) {
            if (error == nil) {
                belluiAuthWebDone(token, authOK, cstr(callbackURL.absoluteString), "");
            } else if ([error.domain isEqualToString:ASWebAuthenticationSessionErrorDomain] &&
                       error.code == ASWebAuthenticationSessionErrorCodeCanceledLogin) {
                belluiAuthWebDone(token, authCancelled, "", "");
            } else {
                NSString *msg = [NSString stringWithFormat:@"%@ (%@ %ld)", error.localizedDescription,
                                                           error.domain, (long)error.code];
                belluiAuthWebDone(token, authFailed, "", cstr(msg));
            }
            drop(token);
          }];
      w.session.presentationContextProvider = w;
      keep(token, w);
      if (![w.session start]) {
          belluiAuthWebDone(token, authFailed, "", "the browser session did not start");
          drop(token);
      }
    });
}

void belluiAuthWebCancel(uint64_t token) {
    dispatch_async(dispatch_get_main_queue(), ^{
      id obj = live[@(token)];
      if ([obj isKindOfClass:[BelluiWebSession class]]) {
          [((BelluiWebSession *)obj).session cancel];
      }
      drop(token);
    });
}

char *belluiAuthBundleID(void) {
    NSString *b = NSBundle.mainBundle.bundleIdentifier;
    return b ? strdup(b.UTF8String) : NULL;
}
