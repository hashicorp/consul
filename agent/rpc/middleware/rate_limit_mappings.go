// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package middleware

import "github.com/hashicorp/consul/agent/consul/rate"

// Maps each net/rpc endpoint to a read or write operation type
// for rate limiting purposes. Please be sure to update this list
// if a net/rpc endpoint is removed.
var rpcRateLimitSpecs = map[string]rate.OperationSpec{
	"ACL.AuthMethodDelete":  {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryACL},
	"ACL.AuthMethodList":    {Type: rate.OperationTypeRead, Category: rate.OperationCategoryACL},
	"ACL.AuthMethodRead":    {Type: rate.OperationTypeRead, Category: rate.OperationCategoryACL},
	"ACL.AuthMethodSet":     {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryACL},
	"ACL.Authorize":         {Type: rate.OperationTypeRead, Category: rate.OperationCategoryACL},
	"ACL.BindingRuleDelete": {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryACL},
	"ACL.BindingRuleList":   {Type: rate.OperationTypeRead, Category: rate.OperationCategoryACL},
	"ACL.BindingRuleRead":   {Type: rate.OperationTypeRead, Category: rate.OperationCategoryACL},
	"ACL.BindingRuleSet":    {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryACL},
	"ACL.BootstrapTokens":   {Type: rate.OperationTypeRead, Category: rate.OperationCategoryACL},
	"ACL.Login":             {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryACL},
	"ACL.Logout":            {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryACL},
	"ACL.PolicyBatchRead":   {Type: rate.OperationTypeRead, Category: rate.OperationCategoryACL},
	"ACL.PolicyDelete":      {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryACL},
	"ACL.PolicyList":        {Type: rate.OperationTypeRead, Category: rate.OperationCategoryACL},
	"ACL.PolicyRead":        {Type: rate.OperationTypeRead, Category: rate.OperationCategoryACL},
	"ACL.PolicyResolve":     {Type: rate.OperationTypeRead, Category: rate.OperationCategoryACL},
	"ACL.PolicySet":         {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryACL},
	"ACL.ReplicationStatus": {Type: rate.OperationTypeRead, Category: rate.OperationCategoryACL},
	"ACL.RoleBatchRead":     {Type: rate.OperationTypeRead, Category: rate.OperationCategoryACL},
	"ACL.RoleDelete":        {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryACL},
	"ACL.RoleList":          {Type: rate.OperationTypeRead, Category: rate.OperationCategoryACL},
	"ACL.RoleRead":          {Type: rate.OperationTypeRead, Category: rate.OperationCategoryACL},
	"ACL.RoleResolve":       {Type: rate.OperationTypeRead, Category: rate.OperationCategoryACL},
	"ACL.RoleSet":           {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryACL},
	"ACL.TokenBatchRead":    {Type: rate.OperationTypeRead, Category: rate.OperationCategoryACL},
	"ACL.TokenClone":        {Type: rate.OperationTypeRead, Category: rate.OperationCategoryACL},
	"ACL.TokenDelete":       {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryACL},
	"ACL.TokenList":         {Type: rate.OperationTypeRead, Category: rate.OperationCategoryACL},
	"ACL.TokenRead":         {Type: rate.OperationTypeRead, Category: rate.OperationCategoryACL},
	"ACL.TokenSet":          {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryACL},

	"AutoConfig.InitialConfiguration": {Type: rate.OperationTypeRead, Category: rate.OperationCategoryAutoConfig},

	"AutoEncrypt.Sign": {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryAutoConfig},

	"Catalog.Deregister":          {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryCatalog},
	"Catalog.GatewayServices":     {Type: rate.OperationTypeRead, Category: rate.OperationCategoryCatalog},
	"Catalog.ListDatacenters":     {Type: rate.OperationTypeRead, Category: rate.OperationCategoryCatalog},
	"Catalog.ListNodes":           {Type: rate.OperationTypeRead, Category: rate.OperationCategoryCatalog},
	"Catalog.ListServices":        {Type: rate.OperationTypeRead, Category: rate.OperationCategoryCatalog},
	"Catalog.NodeServiceList":     {Type: rate.OperationTypeRead, Category: rate.OperationCategoryCatalog},
	"Catalog.NodeServices":        {Type: rate.OperationTypeRead, Category: rate.OperationCategoryCatalog},
	"Catalog.Register":            {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryCatalog},
	"Catalog.ServiceList":         {Type: rate.OperationTypeRead, Category: rate.OperationCategoryCatalog},
	"Catalog.ServiceNodes":        {Type: rate.OperationTypeRead, Category: rate.OperationCategoryCatalog},
	"Catalog.VirtualIPForService": {Type: rate.OperationTypeRead, Category: rate.OperationCategoryCatalog},

	"ConfigEntry.Apply":                {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryConfigEntry},
	"ConfigEntry.Delete":               {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryConfigEntry},
	"ConfigEntry.Get":                  {Type: rate.OperationTypeRead, Category: rate.OperationCategoryConfigEntry},
	"ConfigEntry.List":                 {Type: rate.OperationTypeRead, Category: rate.OperationCategoryConfigEntry},
	"ConfigEntry.ListAll":              {Type: rate.OperationTypeRead, Category: rate.OperationCategoryConfigEntry},
	"ConfigEntry.ResolveServiceConfig": {Type: rate.OperationTypeRead, Category: rate.OperationCategoryConfigEntry},

	"ConnectCA.ConfigurationGet": {Type: rate.OperationTypeRead, Category: rate.OperationCategoryConnectCA},
	"ConnectCA.ConfigurationSet": {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryConnectCA},
	"ConnectCA.Roots":            {Type: rate.OperationTypeRead, Category: rate.OperationCategoryConnectCA},
	"ConnectCA.Sign":             {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryConnectCA},
	"ConnectCA.SignIntermediate": {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryConnectCA},

	"Coordinate.ListDatacenters": {Type: rate.OperationTypeRead, Category: rate.OperationCategoryCoordinate},
	"Coordinate.ListNodes":       {Type: rate.OperationTypeRead, Category: rate.OperationCategoryCoordinate},
	"Coordinate.Node":            {Type: rate.OperationTypeRead, Category: rate.OperationCategoryCoordinate},
	"Coordinate.Update":          {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryCoordinate},

	"DiscoveryChain.Get": {Type: rate.OperationTypeRead, Category: rate.OperationCategoryDiscoveryChain},

	"FederationState.Apply":            {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryFederationState},
	"FederationState.Delete":           {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryFederationState},
	"FederationState.Get":              {Type: rate.OperationTypeRead, Category: rate.OperationCategoryFederationState},
	"FederationState.List":             {Type: rate.OperationTypeRead, Category: rate.OperationCategoryFederationState},
	"FederationState.ListMeshGateways": {Type: rate.OperationTypeRead, Category: rate.OperationCategoryFederationState},

	"Health.ChecksInState": {Type: rate.OperationTypeRead, Category: rate.OperationCategoryHealth},
	"Health.NodeChecks":    {Type: rate.OperationTypeRead, Category: rate.OperationCategoryHealth},
	"Health.ServiceChecks": {Type: rate.OperationTypeRead, Category: rate.OperationCategoryHealth},
	"Health.ServiceNodes":  {Type: rate.OperationTypeRead, Category: rate.OperationCategoryHealth},

	"Intention.Apply": {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryIntention},
	"Intention.Check": {Type: rate.OperationTypeRead, Category: rate.OperationCategoryIntention},
	"Intention.Get":   {Type: rate.OperationTypeRead, Category: rate.OperationCategoryIntention},
	"Intention.List":  {Type: rate.OperationTypeRead, Category: rate.OperationCategoryIntention},
	"Intention.Match": {Type: rate.OperationTypeRead, Category: rate.OperationCategoryIntention},

	"Internal.CatalogOverview":               {Type: rate.OperationTypeRead, Category: rate.OperationCategoryInternal},
	"Internal.EventFire":                     {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryInternal},
	"Internal.ExportedPeeredServices":        {Type: rate.OperationTypeRead, Category: rate.OperationCategoryInternal},
	"Internal.ExportedServicesForPeer":       {Type: rate.OperationTypeRead, Category: rate.OperationCategoryInternal},
	"Internal.GatewayIntentions":             {Type: rate.OperationTypeRead, Category: rate.OperationCategoryInternal},
	"Internal.GatewayServiceDump":            {Type: rate.OperationTypeRead, Category: rate.OperationCategoryInternal},
	"Internal.IntentionUpstreams":            {Type: rate.OperationTypeRead, Category: rate.OperationCategoryInternal},
	"Internal.IntentionUpstreamsDestination": {Type: rate.OperationTypeRead, Category: rate.OperationCategoryInternal},
	"Internal.KeyringOperation":              {Type: rate.OperationTypeRead, Category: rate.OperationCategoryInternal},
	"Internal.NodeDump":                      {Type: rate.OperationTypeRead, Category: rate.OperationCategoryInternal},
	"Internal.NodeInfo":                      {Type: rate.OperationTypeRead, Category: rate.OperationCategoryInternal},
	"Internal.PeeredUpstreams":               {Type: rate.OperationTypeRead, Category: rate.OperationCategoryInternal},
	"Internal.ServiceDump":                   {Type: rate.OperationTypeRead, Category: rate.OperationCategoryInternal},
	"Internal.ServiceGateways":               {Type: rate.OperationTypeRead, Category: rate.OperationCategoryInternal},
	"Internal.ServiceTopology":               {Type: rate.OperationTypeRead, Category: rate.OperationCategoryInternal},

	"KVS.Apply":    {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryKV},
	"KVS.Get":      {Type: rate.OperationTypeRead, Category: rate.OperationCategoryKV},
	"KVS.List":     {Type: rate.OperationTypeRead, Category: rate.OperationCategoryKV},
	"KVS.ListKeys": {Type: rate.OperationTypeRead, Category: rate.OperationCategoryKV},

	"Operator.AutopilotGetConfiguration": {Type: rate.OperationTypeExempt, Category: rate.OperationCategoryOperator},
	"Operator.AutopilotSetConfiguration": {Type: rate.OperationTypeExempt, Category: rate.OperationCategoryOperator},
	"Operator.AutopilotState":            {Type: rate.OperationTypeExempt, Category: rate.OperationCategoryOperator},
	"Operator.RaftGetConfiguration":      {Type: rate.OperationTypeExempt, Category: rate.OperationCategoryOperator},
	"Operator.RaftRemovePeerByAddress":   {Type: rate.OperationTypeExempt, Category: rate.OperationCategoryOperator},
	"Operator.RaftRemovePeerByID":        {Type: rate.OperationTypeExempt, Category: rate.OperationCategoryOperator},
	"Operator.ServerHealth":              {Type: rate.OperationTypeExempt, Category: rate.OperationCategoryOperator},

	"PreparedQuery.Apply":         {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryPreparedQuery},
	"PreparedQuery.Execute":       {Type: rate.OperationTypeRead, Category: rate.OperationCategoryPreparedQuery},
	"PreparedQuery.ExecuteRemote": {Type: rate.OperationTypeRead, Category: rate.OperationCategoryPreparedQuery},
	"PreparedQuery.Explain":       {Type: rate.OperationTypeRead, Category: rate.OperationCategoryPreparedQuery},
	"PreparedQuery.Get":           {Type: rate.OperationTypeRead, Category: rate.OperationCategoryPreparedQuery},
	"PreparedQuery.List":          {Type: rate.OperationTypeRead, Category: rate.OperationCategoryPreparedQuery},

	"Session.Apply":        {Type: rate.OperationTypeWrite, Category: rate.OperationCategorySession},
	"Session.Check":        {Type: rate.OperationTypeRead, Category: rate.OperationCategorySession},
	"Session.Get":          {Type: rate.OperationTypeRead, Category: rate.OperationCategorySession},
	"Session.List":         {Type: rate.OperationTypeRead, Category: rate.OperationCategorySession},
	"Session.NodeSessions": {Type: rate.OperationTypeRead, Category: rate.OperationCategorySession},
	"Session.Renew":        {Type: rate.OperationTypeWrite, Category: rate.OperationCategorySession},

	"Status.Leader":    {Type: rate.OperationTypeExempt, Category: rate.OperationCategoryStatus},
	"Status.Peers":     {Type: rate.OperationTypeExempt, Category: rate.OperationCategoryStatus},
	"Status.Ping":      {Type: rate.OperationTypeExempt, Category: rate.OperationCategoryStatus},
	"Status.RaftStats": {Type: rate.OperationTypeExempt, Category: rate.OperationCategoryStatus},

	"Txn.Apply": {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryTxn},
	"Txn.Read":  {Type: rate.OperationTypeRead, Category: rate.OperationCategoryTxn},

	"Namespace.Write":  {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryPartition},
	"Namespace.Delete": {Type: rate.OperationTypeWrite, Category: rate.OperationCategoryPartition},
	"Namespace.List":   {Type: rate.OperationTypeRead, Category: rate.OperationCategoryPartition},
	"Namespace.Read":   {Type: rate.OperationTypeRead, Category: rate.OperationCategoryPartition},
}
