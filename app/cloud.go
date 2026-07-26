package app

import "github.com/belluikit/bellui"

func (a *belluiApp) SetCloudProvider(p bellui.CloudProvider) {
	if p == nil {
		a.cloud = nil
		return
	}

	a.transitionCloud(p)
}

func (a *belluiApp) transitionCloud(p bellui.CloudProvider) {
	if a.cloud != nil {
		a.cloud.Cleanup(a)
	}

	err := p.Setup(a)
	if err != nil {
		bellui.LogError("Failed to set up cloud provider "+p.ProviderName(), err)
		return
	}
	a.cloud = p

	var listeners []func()
	if cl, ok := a.prefs.(interface{ ChangeListenersCopy() []func() }); ok {
		listeners = cl.ChangeListenersCopy()
	}
	if pp, ok := p.(bellui.CloudProviderPreferences); ok {
		a.prefs = pp.CloudPreferences(a)
	} else {
		a.prefs = a.newDefaultPreferences()
	}
	if cloud, ok := p.(bellui.CloudProviderStorage); ok {
		a.storage = cloud.CloudStorage(a)
	} else {
		store := &store{a: a}
		store.Docs = makeStoreDocs(a.uniqueID, store)
		a.storage = store
	}

	for _, l := range listeners {
		a.prefs.AddChangeListener(l)
		l() // assume that preferences have changed because we replaced the provider
	}

	// after transition ensure settings listener is fired
	a.settings.apply()
}
