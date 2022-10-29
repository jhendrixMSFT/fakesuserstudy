//go:build go1.18
// +build go1.18

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License. See License.txt in the project root for license information.

package fakesuserstudy

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	fakeazcore "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4/fake"
	"github.com/stretchr/testify/require"
)

// matthchr: I don't like that this has to be package-scoped and not scoped to the individual test. There's gonna be a proliferation of these in test files.
type fakeGetServer struct {
	fake.VirtualMachinesServer

	getVMResponse armcompute.VirtualMachine
}

// matthchr: No autocompletion of method name or signature, which is annoying, because no way for me to signify that I want to override a particular method. I need to go
// copy the method from fake.VirtualMachinesServer. Obviously I can do that it'd be nicer if I could intellisense it. Especially because when I copy it I also have to go fix up
// the imports for Responder[...] and ErrorResponder to be fakeazcore.Responder and fakeazcore.ErrorResponder.
// matthchr: Actually, I can't replace with fakeazcore.Responder and fakeazcore.ErrorResponder, I need to replace with fake.Responder and fakeazcore.Responder... why are there two copies?
// The error was:
// have Get(ctx context.Context, resourceGroupName string, vmName string, options *armcompute.VirtualMachinesClientGetOptions) (resp "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake".Responder[armcompute.VirtualMachinesClientGetResponse], err "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake".ErrorResponder)
// want Get(ctx context.Context, resourceGroupName string, vmName string, options *armcompute.VirtualMachinesClientGetOptions) (resp "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4/fake".Responder[armcompute.VirtualMachinesClientGetResponse], err "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4/fake".ErrorResponder)
func (s *fakeGetServer) Get(ctx context.Context, resourceGroupName string, vmName string, options *armcompute.VirtualMachinesClientGetOptions) (resp fake.Responder[armcompute.VirtualMachinesClientGetResponse], err fake.ErrorResponder) {
	// matthchr: Curious why not let me set the response and/or error in the construction of the fakeazcore.Responder?
	// it's pretty minor but IMO this is a bit cleaner as it's a single return statement. I'm not sure if Go generics is smart enough to infer the [T]
	// in the construction but if it is that's nice:
	// return fakeazcore.Responder[armcompute.VirtualMachinesClientGetResponse]{
	//    Resp: armcompute.VirtualMachinesClientGetResponse{
	//		VirtualMachine: s.vm,
	//	}
	// }, fakeazcore.ErrorResponder{}
	// matthchr: I do see for the errorResponder it lets you do errResponder.SetResponseError, which is nice. Maybe just matching that pattern for the standard responder too?
	// matthchr: Ok after using it more, I definitely don't mind the way it's done. It's most awkward for the ErrorResponder and Responder, for the PagerResponder and PollerResonder it works nicer
	responder := fake.Responder[armcompute.VirtualMachinesClientGetResponse]{}
	responder.Set(armcompute.VirtualMachinesClientGetResponse{
		VirtualMachine: s.getVMResponse,
	})
	return responder, fake.ErrorResponder{}
}

func Test_VirtualMachinesClient_Get(t *testing.T) {
	// write a fake for VirtualMachinesClient.Get that satisfies the following requirements

	const (
		vmName            = "virtualmachine1"
		resourceGroupName = "fake-resource-group"
	)

	// the fake VM must return the provided name and its ID contain the provided resource group name.

	responseVM := armcompute.VirtualMachine{
		Name: to.Ptr[string](vmName),
		ID:   to.Ptr[string](resourceGroupName), // This isn't a real ID but good enough for our purposes
	}

	var vm armcompute.VirtualMachine

	// matthchr: It wasn't clear to me until I read the README in armcompute that I needed to embed the fake into my own type,
	// I had guessed at that but then though I was missing something because it's going to make things pretty verbose (need to define a new type for each test).
	// Actually I also have to somehow pass the VM to return up to the server (or hardcode it) which is also unfortunate sorta.
	// matthchr: Have you thought about using lambdas and doing this instead?
	// fakeServer := &fake.VirtualMachinesServer{
	//     // Lambda here can be autocompleted
	//     GetResponse: func abc(ctx context.Context, resourceGroupName string, vmName string, options *armcompute.VirtualMachinesClientGetOptions) (resp fake.Responder[armcompute.VirtualMachinesClientGetResponse], err fake.ErrorResponder) {
	//          // return stuff here -- can capture local test variables if needed to keep stuff scoped to the test
	//     }
	// }
	fakeServer := fakeGetServer{getVMResponse: responseVM}
	client, err := armcompute.NewVirtualMachinesClient("subscriptionID", fakeazcore.NewTokenCredential(), &arm.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Transport: fake.NewVirtualMachinesServerTransport(&fakeServer),
		},
	})
	require.NoError(t, err)

	resp, err := client.Get(context.Background(), resourceGroupName, vmName, nil)
	require.NoError(t, err)
	vm = resp.VirtualMachine

	// the returned VM must satisfy the following conditions
	require.NotNil(t, vm.Name)
	require.Equal(t, vmName, *vm.Name)
	require.NotNil(t, vm.ID)
	require.Contains(t, *vm.ID, resourceGroupName)
}

type fakeDeleteServer struct {
	fake.VirtualMachinesServer
}

func (s *fakeDeleteServer) BeginDelete(ctx context.Context, resourceGroupName string, vmName string, options *armcompute.VirtualMachinesClientBeginDeleteOptions) (resp fake.PollerResponder[armcompute.VirtualMachinesClientDeleteResponse], err fake.ErrorResponder) {

	responder := fake.PollerResponder[armcompute.VirtualMachinesClientDeleteResponse]{}
	responder.AddNonTerminalResponse(nil) // TODO: Do I pass nil here? Unclear
	responder.SetTerminalError("BadRequest", 400)
	// matthchr: Not totally clear to me... does the errorResponder only kick in after PollerResponder has run? Or does it apply before poller?
	errResponder := fake.ErrorResponder{}
	return responder, errResponder
}

func Test_VirtualMachinesClient_BeginDelete(t *testing.T) {
	// write a fake for VirtualMachinesClient.BeginDelete that satisfies the following requirements

	const (
		vmName            = "virtualmachine1"
		resourceGroupName = "fake-resource-group"
	)

	// the fake should include at least one non-terminal response.
	var pollingErr error

	fakeServer := fakeDeleteServer{}
	client, err := armcompute.NewVirtualMachinesClient("subscriptionID", fakeazcore.NewTokenCredential(), &arm.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Transport: fake.NewVirtualMachinesServerTransport(&fakeServer),
		},
	})
	require.NoError(t, err)

	poller, err := client.BeginDelete(context.Background(), resourceGroupName, vmName, nil)

	_, pollingErr = poller.PollUntilDone(context.Background(), nil)

	// the LRO must terminate in a way to satisfy the following conditions
	require.Error(t, pollingErr)
	var respErr *azcore.ResponseError
	require.ErrorAs(t, pollingErr, &respErr)
}

type fakeListServer struct {
	fake.VirtualMachinesServer
}

// matthchr: It's easy to make a copy+paste mistake where I accidentally added this NewListPager to my fakeDeleteServer instead of my fakeListServer, which I then had to debug.
// IMO that's an artifact of 2 things:
// 1. Maybe I didn't need to implement a new server for this case, I could've reused one I already had.
// 2. My fakeDeleteServer wasn't scoped to the test it was for, nor was this fakeListServer, which makes it harder to keep test data separate.
func (s *fakeListServer) NewListPager(resourceGroupName string, options *armcompute.VirtualMachinesClientListOptions) (resp fake.PagerResponder[armcompute.VirtualMachinesClientListResponse]) {
	responder := fake.PagerResponder[armcompute.VirtualMachinesClientListResponse]{}
	responder.AddPage(armcompute.VirtualMachinesClientListResponse{
		VirtualMachineListResult: armcompute.VirtualMachineListResult{
			Value: []*armcompute.VirtualMachine{
				// Empty VMs here
				{},
				{},
			},
		},
	}, nil)
	responder.AddError(fmt.Errorf("oops"))
	responder.AddPage(armcompute.VirtualMachinesClientListResponse{
		VirtualMachineListResult: armcompute.VirtualMachineListResult{
			Value: []*armcompute.VirtualMachine{
				// Empty VMs here
				{},
				{},
				{},
			},
		},
	}, nil)
	return responder
}

func Test_VirtualMachinesClient_NewListPager(t *testing.T) {
	// write a fake for VirtualMachinesClient.NewListPager that satisfies the following requirements

	const (
		resourceGroupName = "fake-resource-group"
	)

	// the fake must return a total of five VMs over two pages.
	// to keep things simple, the returned armcompute.VirtualMachine instances can be empty.
	// while iterating over pages, the fake must return one transient error before the final page

	fakeServer := fakeListServer{}
	client, err := armcompute.NewVirtualMachinesClient("subscriptionID", fakeazcore.NewTokenCredential(), &arm.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Transport: fake.NewVirtualMachinesServerTransport(&fakeServer),
		},
	})
	require.NoError(t, err)

	pager := client.NewListPager(resourceGroupName, nil)

	var vmCount int
	var pageCount int
	var errCount int

	for pager.More() {
		p, err := pager.NextPage(context.Background())
		if err != nil {
			errCount++
			continue
		}

		vmCount += len(p.Value)
		pageCount++
	}

	// the results must satisfy the following conditions
	require.Equal(t, 5, vmCount)
	require.Equal(t, 2, pageCount)
	require.Equal(t, 1, errCount)
}
