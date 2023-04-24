package state

import (
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/proto/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

const (
	testFooPeerID = "9e650110-ac74-4c5a-a6a8-9348b2bed4e9"
	testBarPeerID = "5ebcff30-5509-4858-8142-a8e580f1863f"
	testBazPeerID = "432feb2f-5476-4ae2-b33c-e43640ca0e86"

	testFooSecretID = "e34e9c3d-a27d-4f82-a6d2-28a86af2be6b"
	testBazSecretID = "dd3802bb-0c91-4b2a-be51-505bacae772b"
)

func insertTestPeerings(t *testing.T, s *Store) {
	t.Helper()

	tx := s.db.WriteTxn(0)
	defer tx.Abort()

	err := tx.Insert(tablePeering, &pbpeering.Peering{
		Name:        "foo",
		Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
		ID:          testFooPeerID,
		State:       pbpeering.PeeringState_PENDING,
		CreateIndex: 1,
		ModifyIndex: 1,
	})
	require.NoError(t, err)

	err = tx.Insert(tablePeering, &pbpeering.Peering{
		Name:        "bar",
		Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
		ID:          testBarPeerID,
		State:       pbpeering.PeeringState_FAILING,
		CreateIndex: 2,
		ModifyIndex: 2,
	})
	require.NoError(t, err)

	err = tx.Insert(tableIndex, &IndexEntry{
		Key:   tablePeering,
		Value: 2,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
}

func insertTestPeeringSecret(t *testing.T, s *Store, secret *pbpeering.PeeringSecrets, dialer bool) {
	t.Helper()

	tx := s.db.WriteTxn(0)
	defer tx.Abort()

	err := tx.Insert(tablePeeringSecrets, secret)
	require.NoError(t, err)

	var uuids []string
	if establishment := secret.GetEstablishment().GetSecretID(); establishment != "" {
		uuids = append(uuids, establishment)
	}
	if pending := secret.GetStream().GetPendingSecretID(); pending != "" {
		uuids = append(uuids, pending)
	}
	if active := secret.GetStream().GetActiveSecretID(); active != "" {
		uuids = append(uuids, active)
	}

	// Dialing peers do not track secret UUIDs because they don't generate them.
	if !dialer {
		for _, id := range uuids {
			err = tx.Insert(tablePeeringSecretUUIDs, id)
			require.NoError(t, err)
		}
	}

	require.NoError(t, tx.Commit())
}

func insertTestPeeringTrustBundles(t *testing.T, s *Store) {
	t.Helper()

	tx := s.db.WriteTxn(0)
	defer tx.Abort()

	err := tx.Insert(tablePeeringTrustBundles, &pbpeering.PeeringTrustBundle{
		TrustDomain: "foo.com",
		PeerName:    "foo",
		Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
		RootPEMs:    []string{"foo certificate bundle"},
		CreateIndex: 1,
		ModifyIndex: 1,
	})
	require.NoError(t, err)

	err = tx.Insert(tablePeeringTrustBundles, &pbpeering.PeeringTrustBundle{
		TrustDomain: "bar.com",
		PeerName:    "bar",
		Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
		RootPEMs:    []string{"bar certificate bundle"},
		CreateIndex: 2,
		ModifyIndex: 2,
	})
	require.NoError(t, err)

	err = tx.Insert(tableIndex, &IndexEntry{
		Key:   tablePeeringTrustBundles,
		Value: 2,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
}

func TestStateStore_PeeringReadByID(t *testing.T) {
	s := NewStateStore(nil)
	insertTestPeerings(t, s)

	type testcase struct {
		name   string
		id     string
		expect *pbpeering.Peering
	}
	run := func(t *testing.T, tc testcase) {
		_, peering, err := s.PeeringReadByID(nil, tc.id)
		require.NoError(t, err)
		prototest.AssertDeepEqual(t, tc.expect, peering)
	}
	tcs := []testcase{
		{
			name: "get foo",
			id:   testFooPeerID,
			expect: &pbpeering.Peering{
				Name:        "foo",
				Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				ID:          testFooPeerID,
				State:       pbpeering.PeeringState_PENDING,
				CreateIndex: 1,
				ModifyIndex: 1,
			},
		},
		{
			name: "get bar",
			id:   testBarPeerID,
			expect: &pbpeering.Peering{
				Name:        "bar",
				Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				ID:          testBarPeerID,
				State:       pbpeering.PeeringState_FAILING,
				CreateIndex: 2,
				ModifyIndex: 2,
			},
		},
		{
			name:   "get non-existent",
			id:     "05f54e2f-7813-4d4d-ba03-534554c88a18",
			expect: nil,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestStateStore_PeeringSecretsRead(t *testing.T) {
	s := NewStateStore(nil)

	insertTestPeerings(t, s)

	insertTestPeeringSecret(t, s, &pbpeering.PeeringSecrets{
		PeerID: testFooPeerID,
		Establishment: &pbpeering.PeeringSecrets_Establishment{
			SecretID: testFooSecretID,
		},
	}, false)

	type testcase struct {
		name   string
		peerID string
		expect *pbpeering.PeeringSecrets
	}
	run := func(t *testing.T, tc testcase) {
		secrets, err := s.PeeringSecretsRead(nil, tc.peerID)
		require.NoError(t, err)
		prototest.AssertDeepEqual(t, tc.expect, secrets)
	}
	tcs := []testcase{
		{
			name:   "get foo",
			peerID: testFooPeerID,
			expect: &pbpeering.PeeringSecrets{
				PeerID: testFooPeerID,
				Establishment: &pbpeering.PeeringSecrets_Establishment{
					SecretID: testFooSecretID,
				},
			},
		},
		{
			name:   "get non-existent baz",
			peerID: testBazPeerID,
			expect: nil,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestStore_PeeringSecretsWrite(t *testing.T) {
	dumpUUIDs := func(s *Store) []string {
		tx := s.db.ReadTxn()
		defer tx.Abort()

		iter, err := tx.Get(tablePeeringSecretUUIDs, indexID)
		require.NoError(t, err)

		var resp []string
		for entry := iter.Next(); entry != nil; entry = iter.Next() {
			resp = append(resp, entry.(string))
		}
		return resp
	}

	var (
		testSecretOne   = testUUID()
		testSecretTwo   = testUUID()
		testSecretThree = testUUID()
		testSecretFour  = testUUID()
	)

	type testSeed struct {
		peering *pbpeering.Peering
		secrets *pbpeering.PeeringSecrets
	}

	type testcase struct {
		name        string
		seed        *testSeed
		input       *pbpeering.SecretsWriteRequest
		expect      *pbpeering.PeeringSecrets
		expectUUIDs []string
		expectErr   string
	}

	writeSeed := func(s *Store, seed *testSeed) {
		tx := s.db.WriteTxn(1)
		defer tx.Abort()

		if seed.peering != nil {
			require.NoError(t, tx.Insert(tablePeering, seed.peering))
		}
		if seed.secrets != nil {
			require.NoError(t, tx.Insert(tablePeeringSecrets, seed.secrets))

			var toInsert []string
			if establishment := seed.secrets.GetEstablishment().GetSecretID(); establishment != "" {
				toInsert = append(toInsert, establishment)
			}
			if pending := seed.secrets.GetStream().GetPendingSecretID(); pending != "" {
				toInsert = append(toInsert, pending)
			}
			if active := seed.secrets.GetStream().GetActiveSecretID(); active != "" {
				toInsert = append(toInsert, active)
			}
			for _, id := range toInsert {
				require.NoError(t, tx.Insert(tablePeeringSecretUUIDs, id))
			}
		}

		tx.Commit()
	}

	run := func(t *testing.T, tc testcase) {
		s := NewStateStore(nil)

		// Optionally seed existing secrets for the peering.
		if tc.seed != nil {
			writeSeed(s, tc.seed)
		}

		err := s.PeeringSecretsWrite(10, tc.input)
		if tc.expectErr != "" {
			testutil.RequireErrorContains(t, err, tc.expectErr)
			return
		}
		require.NoError(t, err)

		// Validate that we read what we expect
		secrets, err := s.PeeringSecretsRead(nil, tc.input.GetPeerID())
		require.NoError(t, err)
		require.NotNil(t, secrets)
		prototest.AssertDeepEqual(t, tc.expect, secrets)

		// Validate accounting of the UUIDs table
		require.ElementsMatch(t, tc.expectUUIDs, dumpUUIDs(s))
	}
	tcs := []testcase{
		{
			name: "missing peer id",
			input: &pbpeering.SecretsWriteRequest{
				Request: &pbpeering.SecretsWriteRequest_GenerateToken{},
			},
			expectErr: "missing peer ID",
		},
		{
			name: "unknown peer id",
			input: &pbpeering.SecretsWriteRequest{
				PeerID: testFooPeerID,
				Request: &pbpeering.SecretsWriteRequest_GenerateToken{
					GenerateToken: &pbpeering.SecretsWriteRequest_GenerateTokenRequest{
						EstablishmentSecret: testFooSecretID,
					},
				},
			},
			expectErr: "unknown peering",
		},
		{
			name: "no secret IDs were embedded when generating token",
			input: &pbpeering.SecretsWriteRequest{
				PeerID:  testFooPeerID,
				Request: &pbpeering.SecretsWriteRequest_GenerateToken{},
			},
			expectErr: "missing secret ID",
		},
		{
			name: "no secret IDs were embedded when establishing peering",
			input: &pbpeering.SecretsWriteRequest{
				PeerID:  testFooPeerID,
				Request: &pbpeering.SecretsWriteRequest_Establish{},
			},
			expectErr: "missing secret ID",
		},
		{
			name: "no secret IDs were embedded when exchanging secret",
			input: &pbpeering.SecretsWriteRequest{
				PeerID:  testFooPeerID,
				Request: &pbpeering.SecretsWriteRequest_ExchangeSecret{},
			},
			expectErr: "missing secret ID",
		},
		{
			name: "no secret IDs were embedded when promoting pending secret",
			input: &pbpeering.SecretsWriteRequest{
				PeerID:  testFooPeerID,
				Request: &pbpeering.SecretsWriteRequest_PromotePending{},
			},
			expectErr: "missing secret ID",
		},
		{
			name: "dialing peer invalid request type - generate token",
			seed: &testSeed{
				peering: &pbpeering.Peering{
					Name:                "foo",
					ID:                  testFooPeerID,
					PeerServerAddresses: []string{"10.0.0.1:5300"},
				},
			},
			input: &pbpeering.SecretsWriteRequest{
				PeerID: testFooPeerID,
				// Dialing peer must only write secrets from Establish
				Request: &pbpeering.SecretsWriteRequest_GenerateToken{
					GenerateToken: &pbpeering.SecretsWriteRequest_GenerateTokenRequest{
						EstablishmentSecret: testFooSecretID,
					},
				},
			},
			expectErr: "invalid request type",
		},
		{
			name: "dialing peer invalid request type - exchange secret",
			seed: &testSeed{
				peering: &pbpeering.Peering{
					Name:                "foo",
					ID:                  testFooPeerID,
					PeerServerAddresses: []string{"10.0.0.1:5300"},
				},
			},
			input: &pbpeering.SecretsWriteRequest{
				PeerID: testFooPeerID,
				// Dialing peer must only write secrets from Establish
				Request: &pbpeering.SecretsWriteRequest_ExchangeSecret{
					ExchangeSecret: &pbpeering.SecretsWriteRequest_ExchangeSecretRequest{
						PendingStreamSecret: testFooSecretID,
					},
				},
			},
			expectErr: "invalid request type",
		},
		{
			name: "dialing peer invalid request type - promote pending",
			seed: &testSeed{
				peering: &pbpeering.Peering{
					Name:                "foo",
					ID:                  testFooPeerID,
					PeerServerAddresses: []string{"10.0.0.1:5300"},
				},
			},
			input: &pbpeering.SecretsWriteRequest{
				PeerID: testFooPeerID,
				// Dialing peer must only write secrets from Establish
				Request: &pbpeering.SecretsWriteRequest_PromotePending{
					PromotePending: &pbpeering.SecretsWriteRequest_PromotePendingRequest{
						ActiveStreamSecret: testFooSecretID,
					},
				},
			},
			expectErr: "invalid request type",
		},
		{
			name: "dialing peer does not track UUIDs",
			seed: &testSeed{
				peering: &pbpeering.Peering{
					Name:                "foo",
					ID:                  testFooPeerID,
					PeerServerAddresses: []string{"10.0.0.1:5300"},
				},
			},
			input: &pbpeering.SecretsWriteRequest{
				PeerID: testFooPeerID,
				Request: &pbpeering.SecretsWriteRequest_Establish{
					Establish: &pbpeering.SecretsWriteRequest_EstablishRequest{
						ActiveStreamSecret: testFooSecretID,
					},
				},
			},
			expect: &pbpeering.PeeringSecrets{
				PeerID: testFooPeerID,
				Stream: &pbpeering.PeeringSecrets_Stream{
					ActiveSecretID: testFooSecretID,
				},
			},
			// UUIDs are only tracked for uniqueness in the generating cluster.
			expectUUIDs: []string{},
		},
		{
			name: "generate new establishment secret when secrets already existed",
			seed: &testSeed{
				peering: &pbpeering.Peering{
					Name: "foo",
					ID:   testFooPeerID,
				},
				secrets: &pbpeering.PeeringSecrets{
					PeerID: testFooPeerID,
					Stream: &pbpeering.PeeringSecrets_Stream{
						PendingSecretID: testSecretOne,
						ActiveSecretID:  testSecretTwo,
					},
				},
			},
			input: &pbpeering.SecretsWriteRequest{
				PeerID: testFooPeerID,
				Request: &pbpeering.SecretsWriteRequest_GenerateToken{
					GenerateToken: &pbpeering.SecretsWriteRequest_GenerateTokenRequest{
						EstablishmentSecret: testSecretThree,
					},
				},
			},
			expect: &pbpeering.PeeringSecrets{
				PeerID: testFooPeerID,
				Establishment: &pbpeering.PeeringSecrets_Establishment{
					SecretID: testSecretThree,
				},
				// Stream secrets are inherited
				Stream: &pbpeering.PeeringSecrets_Stream{
					PendingSecretID: testSecretOne,
					ActiveSecretID:  testSecretTwo,
				},
			},
			expectUUIDs: []string{testSecretOne, testSecretTwo, testSecretThree},
		},
		{
			name: "generate new token to replace establishment secret",
			seed: &testSeed{
				peering: &pbpeering.Peering{
					Name: "foo",
					ID:   testFooPeerID,
				},
				secrets: &pbpeering.PeeringSecrets{
					PeerID: testFooPeerID,
					Establishment: &pbpeering.PeeringSecrets_Establishment{
						SecretID: testSecretOne,
					},
				},
			},
			input: &pbpeering.SecretsWriteRequest{
				PeerID: testFooPeerID,
				Request: &pbpeering.SecretsWriteRequest_GenerateToken{
					GenerateToken: &pbpeering.SecretsWriteRequest_GenerateTokenRequest{
						// Two replaces One
						EstablishmentSecret: testSecretTwo,
					},
				},
			},
			expect: &pbpeering.PeeringSecrets{
				PeerID: testFooPeerID,
				Establishment: &pbpeering.PeeringSecrets_Establishment{
					SecretID: testSecretTwo,
				},
			},
			expectUUIDs: []string{testSecretTwo},
		},
		{
			name: "cannot exchange secret without existing secrets",
			seed: &testSeed{
				peering: &pbpeering.Peering{
					Name: "foo",
					ID:   testFooPeerID,
				},
				// Do not seed an establishment secret.
			},
			input: &pbpeering.SecretsWriteRequest{
				PeerID: testFooPeerID,
				Request: &pbpeering.SecretsWriteRequest_ExchangeSecret{
					ExchangeSecret: &pbpeering.SecretsWriteRequest_ExchangeSecretRequest{
						PendingStreamSecret: testSecretOne,
					},
				},
			},
			expectErr: "no known secrets for peering",
		},
		{
			name: "cannot exchange secret without establishment secret",
			seed: &testSeed{
				peering: &pbpeering.Peering{
					Name: "foo",
					ID:   testFooPeerID,
				},
				secrets: &pbpeering.PeeringSecrets{
					PeerID: testFooPeerID,
					Stream: &pbpeering.PeeringSecrets_Stream{
						PendingSecretID: testSecretOne,
					},
				},
			},
			input: &pbpeering.SecretsWriteRequest{
				PeerID: testFooPeerID,
				Request: &pbpeering.SecretsWriteRequest_ExchangeSecret{
					ExchangeSecret: &pbpeering.SecretsWriteRequest_ExchangeSecretRequest{
						// Attempt to replace One with Two
						PendingStreamSecret: testSecretTwo,
					},
				},
			},
			expectErr: "peering was already established",
		},
		{
			name: "cannot exchange secret without valid establishment secret",
			seed: &testSeed{
				peering: &pbpeering.Peering{
					Name: "foo",
					ID:   testFooPeerID,
				},
				secrets: &pbpeering.PeeringSecrets{
					PeerID: testFooPeerID,
					Establishment: &pbpeering.PeeringSecrets_Establishment{
						SecretID: testSecretOne,
					},
				},
			},
			input: &pbpeering.SecretsWriteRequest{
				PeerID: testFooPeerID,
				Request: &pbpeering.SecretsWriteRequest_ExchangeSecret{
					ExchangeSecret: &pbpeering.SecretsWriteRequest_ExchangeSecretRequest{
						// Given secret Three does not match One
						EstablishmentSecret: testSecretThree,
						PendingStreamSecret: testSecretTwo,
					},
				},
			},
			expectErr: "invalid establishment secret",
		},
		{
			name: "exchange secret to generate new pending secret",
			seed: &testSeed{
				peering: &pbpeering.Peering{
					Name: "foo",
					ID:   testFooPeerID,
				},
				secrets: &pbpeering.PeeringSecrets{
					PeerID: testFooPeerID,
					Establishment: &pbpeering.PeeringSecrets_Establishment{
						SecretID: testSecretOne,
					},
				},
			},
			input: &pbpeering.SecretsWriteRequest{
				PeerID: testFooPeerID,
				Request: &pbpeering.SecretsWriteRequest_ExchangeSecret{
					ExchangeSecret: &pbpeering.SecretsWriteRequest_ExchangeSecretRequest{
						EstablishmentSecret: testSecretOne,
						PendingStreamSecret: testSecretTwo,
					},
				},
			},
			expect: &pbpeering.PeeringSecrets{
				PeerID: testFooPeerID,
				Stream: &pbpeering.PeeringSecrets_Stream{
					PendingSecretID: testSecretTwo,
				},
			},
			// Establishment secret testSecretOne is discarded when exchanging for a stream secret
			expectUUIDs: []string{testSecretTwo},
		},
		{
			name: "exchange secret replaces pending stream secret",
			seed: &testSeed{
				peering: &pbpeering.Peering{
					Name: "foo",
					ID:   testFooPeerID,
				},
				secrets: &pbpeering.PeeringSecrets{
					PeerID: testFooPeerID,
					Establishment: &pbpeering.PeeringSecrets_Establishment{
						SecretID: testSecretFour,
					},
					Stream: &pbpeering.PeeringSecrets_Stream{
						ActiveSecretID:  testSecretOne,
						PendingSecretID: testSecretTwo,
					},
				},
			},
			input: &pbpeering.SecretsWriteRequest{
				PeerID: testFooPeerID,
				Request: &pbpeering.SecretsWriteRequest_ExchangeSecret{
					ExchangeSecret: &pbpeering.SecretsWriteRequest_ExchangeSecretRequest{
						EstablishmentSecret: testSecretFour,

						// Three replaces two
						PendingStreamSecret: testSecretThree,
					},
				},
			},
			expect: &pbpeering.PeeringSecrets{
				PeerID: testFooPeerID,
				// Establishment secret is discarded in favor of new pending secret.
				Stream: &pbpeering.PeeringSecrets_Stream{
					// Active secret is not deleted until the new pending secret is promoted
					ActiveSecretID:  testSecretOne,
					PendingSecretID: testSecretThree,
				},
			},
			expectUUIDs: []string{testSecretOne, testSecretThree},
		},
		{
			name: "cannot promote pending without existing secrets",
			seed: &testSeed{
				peering: &pbpeering.Peering{
					Name: "foo",
					ID:   testFooPeerID,
				},
				// Do not seed a pending secret.
			},
			input: &pbpeering.SecretsWriteRequest{
				PeerID: testFooPeerID,
				Request: &pbpeering.SecretsWriteRequest_PromotePending{
					PromotePending: &pbpeering.SecretsWriteRequest_PromotePendingRequest{
						ActiveStreamSecret: testSecretOne,
					},
				},
			},
			expectErr: "no known secrets for peering",
		},
		{
			name: "cannot promote pending without existing pending secret",
			seed: &testSeed{
				peering: &pbpeering.Peering{
					Name: "foo",
					ID:   testFooPeerID,
				},
				secrets: &pbpeering.PeeringSecrets{
					PeerID: testFooPeerID,
					Stream: &pbpeering.PeeringSecrets_Stream{
						ActiveSecretID: testSecretOne,
					},
				},
			},
			input: &pbpeering.SecretsWriteRequest{
				PeerID: testFooPeerID,
				Request: &pbpeering.SecretsWriteRequest_PromotePending{
					PromotePending: &pbpeering.SecretsWriteRequest_PromotePendingRequest{
						// Attempt to replace One with Two
						ActiveStreamSecret: testSecretTwo,
					},
				},
			},
			expectErr: "invalid pending stream secret",
		},
		{
			name: "cannot promote pending without valid pending secret",
			seed: &testSeed{
				peering: &pbpeering.Peering{
					Name: "foo",
					ID:   testFooPeerID,
				},
				secrets: &pbpeering.PeeringSecrets{
					PeerID: testFooPeerID,
					Stream: &pbpeering.PeeringSecrets_Stream{
						PendingSecretID: testSecretTwo,
						ActiveSecretID:  testSecretOne,
					},
				},
			},
			input: &pbpeering.SecretsWriteRequest{
				PeerID: testFooPeerID,
				Request: &pbpeering.SecretsWriteRequest_PromotePending{
					PromotePending: &pbpeering.SecretsWriteRequest_PromotePendingRequest{
						// Attempting to write secret Three, but pending secret is Two
						ActiveStreamSecret: testSecretThree,
					},
				},
			},
			expectErr: "invalid pending stream secret",
		},
		{
			name: "promote pending secret and delete active",
			seed: &testSeed{
				peering: &pbpeering.Peering{
					Name: "foo",
					ID:   testFooPeerID,
				},
				secrets: &pbpeering.PeeringSecrets{
					PeerID: testFooPeerID,
					Establishment: &pbpeering.PeeringSecrets_Establishment{
						SecretID: testSecretThree,
					},
					Stream: &pbpeering.PeeringSecrets_Stream{
						PendingSecretID: testSecretTwo,
						ActiveSecretID:  testSecretOne,
					},
				},
			},
			input: &pbpeering.SecretsWriteRequest{
				PeerID: testFooPeerID,
				Request: &pbpeering.SecretsWriteRequest_PromotePending{
					PromotePending: &pbpeering.SecretsWriteRequest_PromotePendingRequest{
						// Two gets promoted over One
						ActiveStreamSecret: testSecretTwo,
					},
				},
			},
			expect: &pbpeering.PeeringSecrets{
				PeerID: testFooPeerID,
				Establishment: &pbpeering.PeeringSecrets_Establishment{
					// Establishment secret remains valid when promoting a stream secret.
					SecretID: testSecretThree,
				},
				Stream: &pbpeering.PeeringSecrets_Stream{
					ActiveSecretID: testSecretTwo,
				},
			},
			expectUUIDs: []string{testSecretTwo, testSecretThree},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestStore_PeeringSecretsDelete(t *testing.T) {
	const (
		establishmentID = "b4b9cbae-4bbd-454b-b7ae-441a5c89c3b9"
		pendingID       = "0ba06390-bd77-4c52-8397-f88c0867157d"
		activeID        = "0b8a3817-aca0-4c06-94b6-b0763a5cd013"
	)

	type testCase struct {
		dialer bool
		secret *pbpeering.PeeringSecrets
	}

	run := func(t *testing.T, tc testCase) {
		s := NewStateStore(nil)

		insertTestPeerings(t, s)
		insertTestPeeringSecret(t, s, tc.secret, tc.dialer)

		require.NoError(t, s.PeeringSecretsDelete(12, testFooPeerID, tc.dialer))

		// The secrets should be gone
		secrets, err := s.PeeringSecretsRead(nil, testFooPeerID)
		require.NoError(t, err)
		require.Nil(t, secrets)

		uuids := []string{establishmentID, pendingID, activeID}
		for _, id := range uuids {
			free, err := s.ValidateProposedPeeringSecretUUID(id)
			require.NoError(t, err)
			require.True(t, free)
		}
	}

	tt := map[string]testCase{
		"acceptor": {
			dialer: false,
			secret: &pbpeering.PeeringSecrets{
				PeerID: testFooPeerID,
				Establishment: &pbpeering.PeeringSecrets_Establishment{
					SecretID: establishmentID,
				},
				Stream: &pbpeering.PeeringSecrets_Stream{
					PendingSecretID: pendingID,
					ActiveSecretID:  activeID,
				},
			},
		},
		"dialer": {
			dialer: true,
			secret: &pbpeering.PeeringSecrets{
				PeerID: testFooPeerID,
				Stream: &pbpeering.PeeringSecrets_Stream{
					ActiveSecretID: activeID,
				},
			},
		},
	}

	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestStateStore_PeeringRead(t *testing.T) {
	s := NewStateStore(nil)
	insertTestPeerings(t, s)

	type testcase struct {
		name   string
		query  Query
		expect *pbpeering.Peering
	}
	run := func(t *testing.T, tc testcase) {
		_, peering, err := s.PeeringRead(nil, tc.query)
		require.NoError(t, err)
		prototest.AssertDeepEqual(t, tc.expect, peering)
	}
	tcs := []testcase{
		{
			name: "get foo",
			query: Query{
				Value: "foo",
			},
			expect: &pbpeering.Peering{
				Name:        "foo",
				Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				ID:          testFooPeerID,
				State:       pbpeering.PeeringState_PENDING,
				CreateIndex: 1,
				ModifyIndex: 1,
			},
		},
		{
			name: "get non-existent baz",
			query: Query{
				Value: "baz",
			},
			expect: nil,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestStore_Peering_Watch(t *testing.T) {
	s := NewStateStore(nil)

	var lastIdx uint64
	lastIdx++

	// set up initial write
	err := s.PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:   testFooPeerID,
			Name: "foo",
		},
	})
	require.NoError(t, err)

	newWatch := func(t *testing.T, q Query) memdb.WatchSet {
		t.Helper()
		// set up a watch
		ws := memdb.NewWatchSet()

		_, _, err := s.PeeringRead(ws, q)
		require.NoError(t, err)

		return ws
	}

	t.Run("insert fires watch", func(t *testing.T) {
		// watch on non-existent bar
		ws := newWatch(t, Query{Value: "bar"})

		lastIdx++
		err := s.PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{
			Peering: &pbpeering.Peering{
				ID:   testBarPeerID,
				Name: "bar",
			},
		})
		require.NoError(t, err)
		require.True(t, watchFired(ws))

		// should find bar peering
		idx, p, err := s.PeeringRead(ws, Query{Value: "bar"})
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.NotNil(t, p)
	})

	t.Run("update fires watch", func(t *testing.T) {
		// watch on existing foo
		ws := newWatch(t, Query{Value: "foo"})

		// unrelated write shouldn't fire watch
		lastIdx++
		err := s.PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{
			Peering: &pbpeering.Peering{
				ID:   testBarPeerID,
				Name: "bar",
			},
		})
		require.NoError(t, err)
		require.False(t, watchFired(ws))

		// foo write should fire watch
		lastIdx++
		err = s.PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{
			Peering: &pbpeering.Peering{
				ID:        testFooPeerID,
				Name:      "foo",
				State:     pbpeering.PeeringState_DELETING,
				DeletedAt: structs.TimeToProto(time.Now()),
			},
		})
		require.NoError(t, err)
		require.True(t, watchFired(ws))

		// check foo is updated
		idx, p, err := s.PeeringRead(ws, Query{Value: "foo"})
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.False(t, p.IsActive())
	})

	t.Run("delete fires watch", func(t *testing.T) {
		// watch on existing foo
		ws := newWatch(t, Query{Value: "bar"})

		lastIdx++
		require.NoError(t, s.PeeringDelete(lastIdx, Query{Value: "foo"}))
		require.False(t, watchFired(ws))

		// mark for deletion before actually deleting
		lastIdx++
		err := s.PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{Peering: &pbpeering.Peering{
			ID:        testBarPeerID,
			Name:      "bar",
			State:     pbpeering.PeeringState_DELETING,
			DeletedAt: structs.TimeToProto(time.Now()),
		},
		})
		require.NoError(t, err)
		require.True(t, watchFired(ws))

		ws = newWatch(t, Query{Value: "bar"})

		// delete on bar should fire watch
		lastIdx++
		err = s.PeeringDelete(lastIdx, Query{Value: "bar"})
		require.NoError(t, err)
		require.True(t, watchFired(ws))

		// check bar is gone
		idx, p, err := s.PeeringRead(ws, Query{Value: "bar"})
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Nil(t, p)
	})
}

func TestStore_PeeringList(t *testing.T) {
	s := NewStateStore(nil)
	insertTestPeerings(t, s)

	_, pps, err := s.PeeringList(nil, acl.EnterpriseMeta{})
	require.NoError(t, err)
	expect := []*pbpeering.Peering{
		{
			Name:        "foo",
			Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			ID:          testFooPeerID,
			State:       pbpeering.PeeringState_PENDING,
			CreateIndex: 1,
			ModifyIndex: 1,
		},
		{
			Name:        "bar",
			Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			ID:          testBarPeerID,
			State:       pbpeering.PeeringState_FAILING,
			CreateIndex: 2,
			ModifyIndex: 2,
		},
	}
	require.ElementsMatch(t, expect, pps)
}

func TestStore_PeeringList_Watch(t *testing.T) {
	s := NewStateStore(nil)

	var lastIdx uint64
	lastIdx++ // start at 1

	// track number of expected peerings in state store
	var count int

	newWatch := func(t *testing.T, entMeta acl.EnterpriseMeta) memdb.WatchSet {
		t.Helper()
		// set up a watch
		ws := memdb.NewWatchSet()

		_, _, err := s.PeeringList(ws, entMeta)
		require.NoError(t, err)

		return ws
	}

	testutil.RunStep(t, "insert fires watch", func(t *testing.T) {
		ws := newWatch(t, acl.EnterpriseMeta{})

		lastIdx++
		// insert a peering
		err := s.PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{Peering: &pbpeering.Peering{
			ID:        testFooPeerID,
			Name:      "foo",
			Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
		},
		})
		require.NoError(t, err)
		count++

		require.True(t, watchFired(ws))

		// should find bar peering
		idx, pp, err := s.PeeringList(ws, acl.EnterpriseMeta{})
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, pp, count)
	})

	testutil.RunStep(t, "update fires watch", func(t *testing.T) {
		ws := newWatch(t, acl.EnterpriseMeta{})

		// update peering
		lastIdx++
		require.NoError(t, s.PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{
			Peering: &pbpeering.Peering{
				ID:        testFooPeerID,
				Name:      "foo",
				State:     pbpeering.PeeringState_DELETING,
				DeletedAt: structs.TimeToProto(time.Now()),
				Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			},
		}))
		require.True(t, watchFired(ws))

		idx, pp, err := s.PeeringList(ws, acl.EnterpriseMeta{})
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, pp, count)
	})

	testutil.RunStep(t, "delete fires watch", func(t *testing.T) {
		ws := newWatch(t, acl.EnterpriseMeta{})

		// delete peering
		lastIdx++
		err := s.PeeringDelete(lastIdx, Query{Value: "foo"})
		require.NoError(t, err)
		count--

		require.True(t, watchFired(ws))

		idx, pp, err := s.PeeringList(ws, acl.EnterpriseMeta{})
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, pp, count)
	})
}

func TestStore_PeeringWrite(t *testing.T) {
	// Note that all test cases in this test share a state store and must be run sequentially.
	// Each case depends on the previous.
	s := NewStateStore(nil)

	testTime := time.Now()

	type expectations struct {
		peering *pbpeering.Peering
		secrets *pbpeering.PeeringSecrets
		err     string
	}
	type testcase struct {
		name   string
		input  *pbpeering.PeeringWriteRequest
		expect expectations
	}
	run := func(t *testing.T, tc testcase) {
		err := s.PeeringWrite(10, tc.input)
		if tc.expect.err != "" {
			testutil.RequireErrorContains(t, err, tc.expect.err)
			return
		}
		require.NoError(t, err)

		q := Query{
			Value:          tc.input.Peering.Name,
			EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(tc.input.Peering.Partition),
		}
		_, p, err := s.PeeringRead(nil, q)
		require.NoError(t, err)
		require.NotNil(t, p)
		require.Equal(t, tc.expect.peering.State, p.State)
		require.Equal(t, tc.expect.peering.Name, p.Name)
		require.Equal(t, tc.expect.peering.Meta, p.Meta)
		if tc.expect.peering.DeletedAt != nil {
			require.Equal(t, tc.expect.peering.DeletedAt, p.DeletedAt)
		}

		secrets, err := s.PeeringSecretsRead(nil, tc.input.Peering.ID)
		require.NoError(t, err)
		prototest.AssertDeepEqual(t, tc.expect.secrets, secrets)
	}
	tcs := []testcase{
		{
			name: "create baz",
			input: &pbpeering.PeeringWriteRequest{
				Peering: &pbpeering.Peering{
					ID:                  testBazPeerID,
					Name:                "baz",
					State:               pbpeering.PeeringState_ESTABLISHING,
					PeerServerAddresses: []string{"localhost:8502"},
					Partition:           structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				},
				SecretsRequest: &pbpeering.SecretsWriteRequest{
					PeerID: testBazPeerID,
					Request: &pbpeering.SecretsWriteRequest_Establish{
						Establish: &pbpeering.SecretsWriteRequest_EstablishRequest{
							ActiveStreamSecret: testBazSecretID,
						},
					},
				},
			},
			expect: expectations{
				peering: &pbpeering.Peering{
					ID:    testBazPeerID,
					Name:  "baz",
					State: pbpeering.PeeringState_ESTABLISHING,
				},
				secrets: &pbpeering.PeeringSecrets{
					PeerID: testBazPeerID,
					Stream: &pbpeering.PeeringSecrets_Stream{
						ActiveSecretID: testBazSecretID,
					},
				},
			},
		},
		{
			name: "cannot change ID for baz",
			input: &pbpeering.PeeringWriteRequest{
				Peering: &pbpeering.Peering{
					ID:                  "123",
					Name:                "baz",
					State:               pbpeering.PeeringState_FAILING,
					PeerServerAddresses: []string{"localhost:8502"},
					Partition:           structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				},
			},
			expect: expectations{
				err: `A peering already exists with the name "baz" and a different ID`,
			},
		},
		{
			name: "cannot change dialer status for baz",
			input: &pbpeering.PeeringWriteRequest{
				Peering: &pbpeering.Peering{
					ID:    "123",
					Name:  "baz",
					State: pbpeering.PeeringState_FAILING,
					// Excluding the peer server addresses leads to baz not being considered a dialer.
					// PeerServerAddresses: []string{"localhost:8502"},
					Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				},
			},
			expect: expectations{
				err: "Cannot switch peering dialing mode from true to false",
			},
		},
		{
			name: "update baz",
			input: &pbpeering.PeeringWriteRequest{
				Peering: &pbpeering.Peering{
					ID:                  testBazPeerID,
					Name:                "baz",
					State:               pbpeering.PeeringState_FAILING,
					PeerServerAddresses: []string{"localhost:8502"},
					Partition:           structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				},
			},
			expect: expectations{
				peering: &pbpeering.Peering{
					ID:    testBazPeerID,
					Name:  "baz",
					State: pbpeering.PeeringState_FAILING,
				},
				secrets: &pbpeering.PeeringSecrets{
					PeerID: testBazPeerID,
					Stream: &pbpeering.PeeringSecrets_Stream{
						ActiveSecretID: testBazSecretID,
					},
				},
			},
		},
		{
			name: "if no state was included in request it is inherited from existing",
			input: &pbpeering.PeeringWriteRequest{
				Peering: &pbpeering.Peering{
					ID:   testBazPeerID,
					Name: "baz",
					// Send undefined state.
					// State:               pbpeering.PeeringState_FAILING,
					PeerServerAddresses: []string{"localhost:8502"},
					Partition:           structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				},
			},
			expect: expectations{
				peering: &pbpeering.Peering{
					ID:   testBazPeerID,
					Name: "baz",
					// Previous failing state is picked up.
					State: pbpeering.PeeringState_FAILING,
				},
				secrets: &pbpeering.PeeringSecrets{
					PeerID: testBazPeerID,
					Stream: &pbpeering.PeeringSecrets_Stream{
						ActiveSecretID: testBazSecretID,
					},
				},
			},
		},
		{
			name: "mark baz as terminated",
			input: &pbpeering.PeeringWriteRequest{
				Peering: &pbpeering.Peering{
					ID:                  testBazPeerID,
					Name:                "baz",
					State:               pbpeering.PeeringState_TERMINATED,
					PeerServerAddresses: []string{"localhost:8502"},
					Partition:           structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				},
			},
			expect: expectations{
				peering: &pbpeering.Peering{
					ID:    testBazPeerID,
					Name:  "baz",
					State: pbpeering.PeeringState_TERMINATED,
				},
				// Secrets for baz should have been deleted
				secrets: nil,
			},
		},
		{
			name: "cannot modify peering during no-op termination",
			input: &pbpeering.PeeringWriteRequest{
				Peering: &pbpeering.Peering{
					ID:                  testBazPeerID,
					Name:                "baz",
					State:               pbpeering.PeeringState_TERMINATED,
					Partition:           structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
					PeerServerAddresses: []string{"localhost:8502"},

					// Attempt to add metadata
					Meta: map[string]string{"foo": "bar"},
				},
			},
			expect: expectations{
				peering: &pbpeering.Peering{
					ID:    testBazPeerID,
					Name:  "baz",
					State: pbpeering.PeeringState_TERMINATED,
					// Meta should be unchanged.
					Meta: nil,
				},
			},
		},
		{
			name: "mark baz for deletion",
			input: &pbpeering.PeeringWriteRequest{
				Peering: &pbpeering.Peering{
					ID:                  testBazPeerID,
					Name:                "baz",
					State:               pbpeering.PeeringState_DELETING,
					PeerServerAddresses: []string{"localhost:8502"},
					DeletedAt:           structs.TimeToProto(testTime),
					Partition:           structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				},
			},
			expect: expectations{
				peering: &pbpeering.Peering{
					ID:        testBazPeerID,
					Name:      "baz",
					State:     pbpeering.PeeringState_DELETING,
					DeletedAt: structs.TimeToProto(testTime),
				},
				secrets: nil,
			},
		},
		{
			name: "deleting a deleted peering is a no-op",
			input: &pbpeering.PeeringWriteRequest{
				Peering: &pbpeering.Peering{
					ID:                  testBazPeerID,
					Name:                "baz",
					State:               pbpeering.PeeringState_DELETING,
					PeerServerAddresses: []string{"localhost:8502"},
					DeletedAt:           structs.TimeToProto(time.Now()),
					Partition:           structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				},
			},
			expect: expectations{
				peering: &pbpeering.Peering{
					ID:   testBazPeerID,
					Name: "baz",
					// Still marked as deleting at the original testTime
					State:     pbpeering.PeeringState_DELETING,
					DeletedAt: structs.TimeToProto(testTime),
				},
				// Secrets for baz should have been deleted
				secrets: nil,
			},
		},
		{
			name: "terminating a peering marked for deletion is a no-op",
			input: &pbpeering.PeeringWriteRequest{
				Peering: &pbpeering.Peering{
					ID:                  testBazPeerID,
					Name:                "baz",
					State:               pbpeering.PeeringState_TERMINATED,
					PeerServerAddresses: []string{"localhost:8502"},
					Partition:           structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				},
			},
			expect: expectations{
				peering: &pbpeering.Peering{
					ID:   testBazPeerID,
					Name: "baz",
					// Still marked as deleting
					State: pbpeering.PeeringState_DELETING,
				},
				// Secrets for baz should have been deleted
				secrets: nil,
			},
		},
		{
			name: "cannot update peering marked for deletion",
			input: &pbpeering.PeeringWriteRequest{
				Peering: &pbpeering.Peering{
					ID:                  testBazPeerID,
					Name:                "baz",
					PeerServerAddresses: []string{"localhost:8502"},
					Partition:           structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),

					// Attempt to add metadata
					Meta: map[string]string{
						"source": "kubernetes",
					},
				},
			},
			expect: expectations{
				err: "cannot write to peering that is marked for deletion",
			},
		},
		{
			name: "cannot create peering marked for deletion",
			input: &pbpeering.PeeringWriteRequest{
				Peering: &pbpeering.Peering{
					ID:                  testFooPeerID,
					Name:                "foo",
					PeerServerAddresses: []string{"localhost:8502"},
					State:               pbpeering.PeeringState_DELETING,
					DeletedAt:           structs.TimeToProto(time.Now()),
					Partition:           structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				},
			},
			expect: expectations{
				err: "cannot create a new peering marked for deletion",
			},
		},
	}
	for _, tc := range tcs {
		testutil.RunStep(t, tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestStore_PeeringDelete(t *testing.T) {
	s := NewStateStore(nil)
	insertTestPeerings(t, s)

	testutil.RunStep(t, "cannot delete without marking for deletion", func(t *testing.T) {
		q := Query{Value: "foo"}
		err := s.PeeringDelete(10, q)
		testutil.RequireErrorContains(t, err, "cannot delete a peering without first marking for deletion")
	})

	testutil.RunStep(t, "can delete after marking for deletion", func(t *testing.T) {
		require.NoError(t, s.PeeringWrite(11, &pbpeering.PeeringWriteRequest{
			Peering: &pbpeering.Peering{
				ID:        testFooPeerID,
				Name:      "foo",
				State:     pbpeering.PeeringState_DELETING,
				DeletedAt: structs.TimeToProto(time.Now()),
			},
		}))

		q := Query{Value: "foo"}
		require.NoError(t, s.PeeringDelete(12, q))

		_, p, err := s.PeeringRead(nil, q)
		require.NoError(t, err)
		require.Nil(t, p)
	})
}

func TestStore_PeeringTerminateByID(t *testing.T) {
	s := NewStateStore(nil)
	insertTestPeerings(t, s)

	// id corresponding to default/foo
	const id = testFooPeerID

	require.NoError(t, s.PeeringTerminateByID(10, id))

	_, p, err := s.PeeringReadByID(nil, id)
	require.NoError(t, err)
	require.Equal(t, pbpeering.PeeringState_TERMINATED, p.State)
}

func TestStateStore_PeeringTrustBundleList(t *testing.T) {
	s := NewStateStore(nil)
	insertTestPeeringTrustBundles(t, s)

	type testcase struct {
		name    string
		entMeta acl.EnterpriseMeta
		expect  []*pbpeering.PeeringTrustBundle
	}

	entMeta := structs.NodeEnterpriseMetaInDefaultPartition()

	expect := []*pbpeering.PeeringTrustBundle{
		{
			TrustDomain: "bar.com",
			PeerName:    "bar",
			Partition:   entMeta.PartitionOrEmpty(),
			RootPEMs:    []string{"bar certificate bundle"},
			CreateIndex: 2,
			ModifyIndex: 2,
		},
		{
			TrustDomain: "foo.com",
			PeerName:    "foo",
			Partition:   entMeta.PartitionOrEmpty(),
			RootPEMs:    []string{"foo certificate bundle"},
			CreateIndex: 1,
			ModifyIndex: 1,
		},
	}

	_, bundles, err := s.PeeringTrustBundleList(nil, *entMeta)
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, expect, bundles)
}

func TestStateStore_PeeringTrustBundleRead(t *testing.T) {
	s := NewStateStore(nil)
	insertTestPeeringTrustBundles(t, s)

	type testcase struct {
		name   string
		query  Query
		expect *pbpeering.PeeringTrustBundle
	}
	run := func(t *testing.T, tc testcase) {
		_, ptb, err := s.PeeringTrustBundleRead(nil, tc.query)
		require.NoError(t, err)
		prototest.AssertDeepEqual(t, tc.expect, ptb)
	}

	entMeta := structs.NodeEnterpriseMetaInDefaultPartition()

	tcs := []testcase{
		{
			name: "get foo",
			query: Query{
				Value:          "foo",
				EnterpriseMeta: *entMeta,
			},
			expect: &pbpeering.PeeringTrustBundle{
				TrustDomain: "foo.com",
				PeerName:    "foo",
				Partition:   entMeta.PartitionOrEmpty(),
				RootPEMs:    []string{"foo certificate bundle"},
				CreateIndex: 1,
				ModifyIndex: 1,
			},
		},
		{
			name: "get non-existent baz",
			query: Query{
				Value: "baz",
			},
			expect: nil,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestStore_PeeringTrustBundleWrite(t *testing.T) {
	s := NewStateStore(nil)
	insertTestPeeringTrustBundles(t, s)
	type testcase struct {
		name  string
		input *pbpeering.PeeringTrustBundle
	}
	run := func(t *testing.T, tc testcase) {
		require.NoError(t, s.PeeringTrustBundleWrite(10, tc.input))

		q := Query{
			Value:          tc.input.PeerName,
			EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(tc.input.Partition),
		}
		_, ptb, err := s.PeeringTrustBundleRead(nil, q)
		require.NoError(t, err)
		require.NotNil(t, ptb)
		require.Equal(t, tc.input.TrustDomain, ptb.TrustDomain)
		require.Equal(t, tc.input.PeerName, ptb.PeerName)
	}
	tcs := []testcase{
		{
			name: "create baz",
			input: &pbpeering.PeeringTrustBundle{
				TrustDomain: "baz.com",
				PeerName:    "baz",
				Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			},
		},
		{
			name: "update foo",
			input: &pbpeering.PeeringTrustBundle{
				TrustDomain: "foo-updated.com",
				PeerName:    "foo",
				Partition:   structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestStore_PeeringTrustBundleDelete(t *testing.T) {
	s := NewStateStore(nil)
	insertTestPeeringTrustBundles(t, s)

	q := Query{Value: "foo"}

	require.NoError(t, s.PeeringTrustBundleDelete(10, q))

	_, ptb, err := s.PeeringRead(nil, q)
	require.NoError(t, err)
	require.Nil(t, ptb)
}

func TestStateStore_ExportedServicesForPeer(t *testing.T) {
	s := NewStateStore(nil)

	var lastIdx uint64

	ca := &structs.CAConfiguration{
		Provider:  "consul",
		ClusterID: connect.TestClusterID,
	}
	lastIdx++
	require.NoError(t, s.CASetConfig(lastIdx, ca))

	lastIdx++
	require.NoError(t, s.PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:   testUUID(),
			Name: "my-peering",
		},
	}))

	_, p, err := s.PeeringRead(nil, Query{
		Value: "my-peering",
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	id := p.ID

	defaultEntMeta := structs.DefaultEnterpriseMetaInDefaultPartition()

	newSN := func(name string) structs.ServiceName {
		return structs.NewServiceName(name, defaultEntMeta)
	}

	ws := memdb.NewWatchSet()

	ensureConfigEntry := func(t *testing.T, entry structs.ConfigEntry) {
		t.Helper()
		require.NoError(t, entry.Normalize())
		require.NoError(t, entry.Validate())

		lastIdx++
		require.NoError(t, s.EnsureConfigEntry(lastIdx, entry))
	}

	newTarget := func(service, serviceSubset, datacenter string) *structs.DiscoveryTarget {
		t := structs.NewDiscoveryTarget(service, serviceSubset, "default", "default", datacenter)
		t.SNI = connect.TargetSNI(t, connect.TestTrustDomain)
		t.Name = t.SNI
		t.ConnectTimeout = 5 * time.Second // default
		return t
	}

	testutil.RunStep(t, "no exported services", func(t *testing.T) {
		expect := &structs.ExportedServiceList{}

		idx, got, err := s.ExportedServicesForPeer(ws, id, "dc1")
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Equal(t, expect, got)
	})

	testutil.RunStep(t, "config entry with exact service names", func(t *testing.T) {
		entry := &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "mysql",
					Consumers: []structs.ServiceConsumer{
						{PeerName: "my-peering"},
					},
				},
				{
					Name: "redis",
					Consumers: []structs.ServiceConsumer{
						{PeerName: "my-peering"},
					},
				},
				{
					Name: "mongo",
					Consumers: []structs.ServiceConsumer{
						{PeerName: "my-other-peering"},
					},
				},
			},
		}
		ensureConfigEntry(t, entry)

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		expect := &structs.ExportedServiceList{
			Services: []structs.ServiceName{
				{
					Name:           "mysql",
					EnterpriseMeta: *defaultEntMeta,
				},
				{
					Name:           "redis",
					EnterpriseMeta: *defaultEntMeta,
				},
			},
			DiscoChains: map[structs.ServiceName]structs.ExportedDiscoveryChainInfo{
				newSN("mysql"): {
					Protocol: "tcp",
					TCPTargets: []*structs.DiscoveryTarget{
						newTarget("mysql", "", "dc1"),
					},
				},
				newSN("redis"): {
					Protocol: "tcp",
					TCPTargets: []*structs.DiscoveryTarget{
						newTarget("redis", "", "dc1"),
					},
				},
			},
		}

		idx, got, err := s.ExportedServicesForPeer(ws, id, "dc1")
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Equal(t, expect, got)
	})

	testutil.RunStep(t, "config entry with wildcard service name picks up existing service", func(t *testing.T) {
		lastIdx++
		require.NoError(t, s.EnsureNode(lastIdx, &structs.Node{
			Node: "foo", Address: "127.0.0.1",
		}))

		lastIdx++
		require.NoError(t, s.EnsureService(lastIdx, "foo", &structs.NodeService{
			ID: "billing", Service: "billing", Port: 5000,
		}))

		entry := &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "*",
					Consumers: []structs.ServiceConsumer{
						{PeerName: "my-peering"},
					},
				},
			},
		}
		ensureConfigEntry(t, entry)

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		expect := &structs.ExportedServiceList{
			Services: []structs.ServiceName{
				{
					Name:           "billing",
					EnterpriseMeta: *defaultEntMeta,
				},
			},
			DiscoChains: map[structs.ServiceName]structs.ExportedDiscoveryChainInfo{
				newSN("billing"): {
					Protocol: "tcp",
					TCPTargets: []*structs.DiscoveryTarget{
						newTarget("billing", "", "dc1"),
					},
				},
			},
		}
		idx, got, err := s.ExportedServicesForPeer(ws, id, "dc1")
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Equal(t, expect, got)
	})

	testutil.RunStep(t, "config entry with wildcard service names picks up new registrations", func(t *testing.T) {
		lastIdx++
		require.NoError(t, s.EnsureService(lastIdx, "foo", &structs.NodeService{
			ID: "payments", Service: "payments", Port: 5000,
		}))

		// The proxy will be ignored.
		lastIdx++
		require.NoError(t, s.EnsureService(lastIdx, "foo", &structs.NodeService{
			Kind:    structs.ServiceKindConnectProxy,
			ID:      "payments-proxy",
			Service: "payments-proxy",
			Port:    5000,
		}))

		// Ensure everything is L7-capable.
		ensureConfigEntry(t, &structs.ProxyConfigEntry{
			Kind: structs.ProxyDefaults,
			Name: structs.ProxyConfigGlobal,
			Config: map[string]interface{}{
				"protocol": "http",
			},
			EnterpriseMeta: *defaultEntMeta,
		})

		ensureConfigEntry(t, &structs.ServiceRouterConfigEntry{
			Kind:           structs.ServiceRouter,
			Name:           "router",
			EnterpriseMeta: *defaultEntMeta,
		})

		ensureConfigEntry(t, &structs.ServiceSplitterConfigEntry{
			Kind:           structs.ServiceSplitter,
			Name:           "splitter",
			EnterpriseMeta: *defaultEntMeta,
			Splits:         []structs.ServiceSplit{{Weight: 100}},
		})

		ensureConfigEntry(t, &structs.ServiceResolverConfigEntry{
			Kind:           structs.ServiceResolver,
			Name:           "resolver",
			EnterpriseMeta: *defaultEntMeta,
		})

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		expect := &structs.ExportedServiceList{
			Services: []structs.ServiceName{
				{
					Name:           "billing",
					EnterpriseMeta: *defaultEntMeta,
				},
				{
					Name:           "payments",
					EnterpriseMeta: *defaultEntMeta,
				},
				// NOTE: no payments-proxy here
			},
			DiscoChains: map[structs.ServiceName]structs.ExportedDiscoveryChainInfo{
				newSN("billing"): {
					Protocol: "http",
				},
				newSN("payments"): {
					Protocol: "http",
				},
				newSN("resolver"): {
					Protocol: "http",
				},
				newSN("router"): {
					Protocol: "http",
				},
				newSN("splitter"): {
					Protocol: "http",
				},
			},
		}
		idx, got, err := s.ExportedServicesForPeer(ws, id, "dc1")
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Equal(t, expect, got)
	})

	testutil.RunStep(t, "config entry with wildcard service names picks up service deletions", func(t *testing.T) {
		lastIdx++
		require.NoError(t, s.DeleteService(lastIdx, "foo", "billing", nil, ""))

		lastIdx++
		require.NoError(t, s.DeleteConfigEntry(lastIdx, structs.ServiceSplitter, "splitter", nil))

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		expect := &structs.ExportedServiceList{
			Services: []structs.ServiceName{
				{
					Name:           "payments",
					EnterpriseMeta: *defaultEntMeta,
				},
				// NOTE: no payments-proxy here
			},
			DiscoChains: map[structs.ServiceName]structs.ExportedDiscoveryChainInfo{
				newSN("payments"): {
					Protocol: "http",
				},
				newSN("resolver"): {
					Protocol: "http",
				},
				newSN("router"): {
					Protocol: "http",
				},
			},
		}
		idx, got, err := s.ExportedServicesForPeer(ws, id, "dc1")
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Equal(t, expect, got)
	})

	testutil.RunStep(t, "deleting the config entry clears exported services", func(t *testing.T) {
		expect := &structs.ExportedServiceList{}

		require.NoError(t, s.DeleteConfigEntry(lastIdx, structs.ExportedServices, "default", defaultEntMeta))
		idx, got, err := s.ExportedServicesForPeer(ws, id, "dc1")
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Equal(t, expect, got)
	})
}

func TestStateStore_PeeringsForService(t *testing.T) {
	type testPeering struct {
		peering *pbpeering.Peering
		delete  bool
	}
	type testCase struct {
		name      string
		services  []structs.ServiceName
		peerings  []testPeering
		entry     *structs.ExportedServicesConfigEntry
		query     []string
		expect    [][]*pbpeering.Peering
		expectIdx uint64
	}

	run := func(t *testing.T, tc testCase) {
		s := testStateStore(t)

		var lastIdx uint64
		// Create peerings
		for _, tp := range tc.peerings {
			if tp.peering.ID == "" {
				tp.peering.ID = testUUID()
			}
			lastIdx++
			require.NoError(t, s.PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{Peering: tp.peering}))

			// New peerings can't be marked for deletion so there is a two step process
			// of first creating the peering and then marking it for deletion by setting DeletedAt.
			if tp.delete {
				lastIdx++

				copied := pbpeering.Peering{
					ID:        tp.peering.ID,
					Name:      tp.peering.Name,
					State:     pbpeering.PeeringState_DELETING,
					DeletedAt: structs.TimeToProto(time.Now()),
				}
				require.NoError(t, s.PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{Peering: &copied}))
			}

			// make sure it got created
			q := Query{Value: tp.peering.Name}
			_, p, err := s.PeeringRead(nil, q)
			require.NoError(t, err)
			require.NotNil(t, p)
		}

		// Create a Nodes for services
		svcNode := &structs.Node{Node: "foo", Address: "127.0.0.1"}
		lastIdx++
		require.NoError(t, s.EnsureNode(lastIdx, svcNode))

		// Create the test services
		for _, svc := range tc.services {
			lastIdx++
			require.NoError(t, s.EnsureService(lastIdx, svcNode.Node, &structs.NodeService{
				ID:      svc.Name,
				Service: svc.Name,
				Port:    8080,
			}))
		}

		// Write the config entries.
		if tc.entry != nil {
			lastIdx++
			require.NoError(t, tc.entry.Normalize())
			require.NoError(t, s.EnsureConfigEntry(lastIdx, tc.entry))
		}

		// Query for peers.
		for resultIdx, q := range tc.query {
			tx := s.db.ReadTxn()
			defer tx.Abort()
			idx, peers, err := s.PeeringsForService(nil, q, *acl.DefaultEnterpriseMeta())
			require.NoError(t, err)
			require.Equal(t, tc.expectIdx, idx)

			// Verify the result, ignoring generated fields
			require.Len(t, peers, len(tc.expect[resultIdx]))
			for _, got := range peers {
				got.ID = ""
				got.ModifyIndex = 0
				got.CreateIndex = 0
			}
			require.ElementsMatch(t, tc.expect[resultIdx], peers)
		}
	}

	cases := []testCase{
		{
			name: "no exported services",
			services: []structs.ServiceName{
				{Name: "foo"},
			},
			peerings: []testPeering{},
			entry:    nil,
			query:    []string{"foo"},
			expect:   [][]*pbpeering.Peering{{}},
		},
		{
			name: "peerings marked for deletion are excluded",
			services: []structs.ServiceName{
				{Name: "foo"},
			},
			peerings: []testPeering{
				{
					peering: &pbpeering.Peering{
						Name:  "peer1",
						State: pbpeering.PeeringState_PENDING,
					},
				},
				{
					peering: &pbpeering.Peering{
						Name: "peer2",
					},
					delete: true,
				},
			},
			entry: &structs.ExportedServicesConfigEntry{
				Name: "default",
				Services: []structs.ExportedService{
					{
						Name: "foo",
						Consumers: []structs.ServiceConsumer{
							{
								PeerName: "peer1",
							},
							{
								PeerName: "peer2",
							},
						},
					},
				},
			},
			query: []string{"foo"},
			expect: [][]*pbpeering.Peering{
				{
					{Name: "peer1", State: pbpeering.PeeringState_PENDING},
				},
			},
			expectIdx: uint64(6), // config	entries max index
		},
		{
			name: "config entry with exact service name",
			services: []structs.ServiceName{
				{Name: "foo"},
				{Name: "bar"},
			},
			peerings: []testPeering{
				{
					peering: &pbpeering.Peering{
						Name:  "peer1",
						State: pbpeering.PeeringState_PENDING,
					},
				},
				{
					peering: &pbpeering.Peering{
						Name:  "peer2",
						State: pbpeering.PeeringState_PENDING,
					},
				},
			},
			entry: &structs.ExportedServicesConfigEntry{
				Name: "default",
				Services: []structs.ExportedService{
					{
						Name: "foo",
						Consumers: []structs.ServiceConsumer{
							{
								PeerName: "peer1",
							},
						},
					},
					{
						Name: "bar",
						Consumers: []structs.ServiceConsumer{
							{
								PeerName: "peer2",
							},
						},
					},
				},
			},
			query: []string{"foo", "bar"},
			expect: [][]*pbpeering.Peering{
				{
					{Name: "peer1", State: pbpeering.PeeringState_PENDING},
				},
				{
					{Name: "peer2", State: pbpeering.PeeringState_PENDING},
				},
			},
			expectIdx: uint64(6), // config	entries max index
		},
		{
			name: "config entry with wildcard service name",
			services: []structs.ServiceName{
				{Name: "foo"},
				{Name: "bar"},
			},
			peerings: []testPeering{
				{
					peering: &pbpeering.Peering{
						Name:  "peer1",
						State: pbpeering.PeeringState_PENDING,
					},
				},
				{
					peering: &pbpeering.Peering{
						Name:  "peer2",
						State: pbpeering.PeeringState_PENDING,
					},
				},
				{
					peering: &pbpeering.Peering{
						Name:  "peer3",
						State: pbpeering.PeeringState_PENDING,
					},
				},
			},
			entry: &structs.ExportedServicesConfigEntry{
				Name: "default",
				Services: []structs.ExportedService{
					{
						Name: "*",
						Consumers: []structs.ServiceConsumer{
							{
								PeerName: "peer1",
							},
							{
								PeerName: "peer2",
							},
						},
					},
					{
						Name: "bar",
						Consumers: []structs.ServiceConsumer{
							{
								PeerName: "peer3",
							},
						},
					},
				},
			},
			query: []string{"foo", "bar"},
			expect: [][]*pbpeering.Peering{
				{
					{Name: "peer1", State: pbpeering.PeeringState_PENDING},
					{Name: "peer2", State: pbpeering.PeeringState_PENDING},
				},
				{
					{Name: "peer3", State: pbpeering.PeeringState_PENDING},
				},
			},
			expectIdx: uint64(7),
		},
	}

	for _, tc := range cases {
		testutil.RunStep(t, tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestStore_TrustBundleListByService(t *testing.T) {
	store := testStateStore(t)
	entMeta := *acl.DefaultEnterpriseMeta()

	var lastIdx uint64

	ca := &structs.CAConfiguration{
		Provider:  "consul",
		ClusterID: connect.TestClusterID,
	}
	lastIdx++
	require.NoError(t, store.CASetConfig(lastIdx, ca))

	var (
		peerID1 = testUUID()
		peerID2 = testUUID()
	)

	ws := memdb.NewWatchSet()
	testutil.RunStep(t, "no results on initial setup", func(t *testing.T) {
		idx, resp, err := store.TrustBundleListByService(ws, "foo", "dc1", entMeta)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, resp, 0)
	})

	testutil.RunStep(t, "registering service does not yield trust bundles", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.EnsureNode(lastIdx, &structs.Node{
			Node:    "my-node",
			Address: "127.0.0.1",
		}))

		lastIdx++
		require.NoError(t, store.EnsureService(lastIdx, "my-node", &structs.NodeService{
			ID:      "foo-1",
			Service: "foo",
			Port:    8000,
		}))

		require.False(t, watchFired(ws))

		idx, resp, err := store.TrustBundleListByService(ws, "foo", "dc1", entMeta)
		require.NoError(t, err)
		require.Len(t, resp, 0)
		require.Equal(t, lastIdx-2, idx)
	})

	testutil.RunStep(t, "creating peering does not yield trust bundles", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{
			Peering: &pbpeering.Peering{
				ID:   peerID1,
				Name: "peer1",
			},
		}))

		// The peering is only watched after the service is exported via config entry.
		require.False(t, watchFired(ws))

		idx, resp, err := store.TrustBundleListByService(ws, "foo", "dc1", entMeta)
		require.NoError(t, err)
		require.Len(t, resp, 0)
		require.Equal(t, lastIdx-3, idx)
	})

	testutil.RunStep(t, "exporting the service does not yield trust bundles", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.EnsureConfigEntry(lastIdx, &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "foo",
					Consumers: []structs.ServiceConsumer{
						{
							PeerName: "peer1",
						},
					},
				},
			},
		}))

		// The config entry is watched.
		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		idx, resp, err := store.TrustBundleListByService(ws, "foo", "dc1", entMeta)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, resp, 0)
	})

	testutil.RunStep(t, "trust bundles are returned after they are created", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.PeeringTrustBundleWrite(lastIdx, &pbpeering.PeeringTrustBundle{
			TrustDomain: "peer1.com",
			PeerName:    "peer1",
			RootPEMs:    []string{"peer-root-1"},
		}))

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		idx, resp, err := store.TrustBundleListByService(ws, "foo", "dc1", entMeta)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, resp, 1)
		require.Equal(t, []string{"peer-root-1"}, resp[0].RootPEMs)
	})

	testutil.RunStep(t, "trust bundles are not returned after unexporting service", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.DeleteConfigEntry(lastIdx, structs.ExportedServices, "default", &entMeta))

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		idx, resp, err := store.TrustBundleListByService(ws, "foo", "dc1", entMeta)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, resp, 0)
	})

	testutil.RunStep(t, "trust bundles are returned after config entry is restored", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.EnsureConfigEntry(lastIdx, &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "foo",
					Consumers: []structs.ServiceConsumer{
						{
							PeerName: "peer1",
						},
					},
				},
			},
		}))

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		idx, resp, err := store.TrustBundleListByService(ws, "foo", "dc1", entMeta)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, resp, 1)
		require.Equal(t, []string{"peer-root-1"}, resp[0].RootPEMs)
	})

	testutil.RunStep(t, "bundles for other peers are ignored", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{
			Peering: &pbpeering.Peering{
				ID:   peerID2,
				Name: "peer2",
			},
		}))

		lastIdx++
		require.NoError(t, store.PeeringTrustBundleWrite(lastIdx, &pbpeering.PeeringTrustBundle{
			TrustDomain: "peer2.com",
			PeerName:    "peer2",
			RootPEMs:    []string{"peer-root-2"},
		}))

		// No relevant changes.
		require.False(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		idx, resp, err := store.TrustBundleListByService(ws, "foo", "dc1", entMeta)
		require.NoError(t, err)
		require.Equal(t, lastIdx-2, idx)
		require.Len(t, resp, 1)
		require.Equal(t, []string{"peer-root-1"}, resp[0].RootPEMs)
	})

	testutil.RunStep(t, "second bundle is returned when service is exported to that peer", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.EnsureConfigEntry(lastIdx, &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "foo",
					Consumers: []structs.ServiceConsumer{
						{
							PeerName: "peer1",
						},
						{
							PeerName: "peer2",
						},
					},
				},
			},
		}))

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		idx, resp, err := store.TrustBundleListByService(ws, "foo", "dc1", entMeta)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, resp, 2)
		require.Equal(t, []string{"peer-root-1"}, resp[0].RootPEMs)
		require.Equal(t, []string{"peer-root-2"}, resp[1].RootPEMs)
	})

	testutil.RunStep(t, "deleting the peering excludes its trust bundle", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.PeeringWrite(lastIdx, &pbpeering.PeeringWriteRequest{
			Peering: &pbpeering.Peering{
				ID:        peerID1,
				Name:      "peer1",
				State:     pbpeering.PeeringState_DELETING,
				DeletedAt: structs.TimeToProto(time.Now()),
			},
		}))

		require.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()

		idx, resp, err := store.TrustBundleListByService(ws, "foo", "dc1", entMeta)
		require.NoError(t, err)
		require.Equal(t, lastIdx, idx)
		require.Len(t, resp, 1)
		require.Equal(t, []string{"peer-root-2"}, resp[0].RootPEMs)
	})

	testutil.RunStep(t, "deleting the service does not excludes its trust bundle", func(t *testing.T) {
		lastIdx++
		require.NoError(t, store.DeleteService(lastIdx, "my-node", "foo-1", &entMeta, ""))

		require.False(t, watchFired(ws))

		idx, resp, err := store.TrustBundleListByService(ws, "foo", "dc1", entMeta)
		require.NoError(t, err)
		require.Equal(t, lastIdx-1, idx)
		require.Len(t, resp, 1)
		require.Equal(t, []string{"peer-root-2"}, resp[0].RootPEMs)
	})
}

func TestStateStore_Peering_ListDeleted(t *testing.T) {
	s := testStateStore(t)

	// Insert one active peering and two marked for deletion.
	{
		tx := s.db.WriteTxn(0)
		defer tx.Abort()

		err := tx.Insert(tablePeering, &pbpeering.Peering{
			Name:        "foo",
			Partition:   acl.DefaultPartitionName,
			ID:          testFooPeerID,
			DeletedAt:   structs.TimeToProto(time.Now()),
			CreateIndex: 1,
			ModifyIndex: 1,
		})
		require.NoError(t, err)

		err = tx.Insert(tablePeering, &pbpeering.Peering{
			Name:        "bar",
			Partition:   acl.DefaultPartitionName,
			ID:          testBarPeerID,
			CreateIndex: 2,
			ModifyIndex: 2,
		})
		require.NoError(t, err)

		err = tx.Insert(tablePeering, &pbpeering.Peering{
			Name:        "baz",
			Partition:   acl.DefaultPartitionName,
			ID:          testBazPeerID,
			DeletedAt:   structs.TimeToProto(time.Now()),
			CreateIndex: 3,
			ModifyIndex: 3,
		})
		require.NoError(t, err)

		err = tx.Insert(tableIndex, &IndexEntry{
			Key:   tablePeering,
			Value: 3,
		})
		require.NoError(t, err)
		require.NoError(t, tx.Commit())

	}

	idx, deleted, err := s.PeeringListDeleted(nil)
	require.NoError(t, err)
	require.Equal(t, uint64(3), idx)
	require.Len(t, deleted, 2)

	var names []string
	for _, peering := range deleted {
		names = append(names, peering.Name)
	}

	require.ElementsMatch(t, []string{"foo", "baz"}, names)
}
