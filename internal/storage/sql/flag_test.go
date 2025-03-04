package sql_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"go.flipt.io/flipt/internal/storage"
	fliptsql "go.flipt.io/flipt/internal/storage/sql"
	"go.flipt.io/flipt/internal/storage/sql/common"
	flipt "go.flipt.io/flipt/rpc/flipt"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s *DBTestSuite) TestGetFlag() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		Key:         t.Name(),
		Name:        "foo",
		Description: "bar",
		Enabled:     true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag)

	got, err := s.store.GetFlag(context.TODO(), storage.DefaultNamespace, flag.Key)

	require.NoError(t, err)
	assert.NotNil(t, got)

	assert.Equal(t, storage.DefaultNamespace, got.NamespaceKey)
	assert.Equal(t, flag.Key, got.Key)
	assert.Equal(t, flag.Name, got.Name)
	assert.Equal(t, flag.Description, got.Description)
	assert.Equal(t, flag.Enabled, got.Enabled)
	assert.NotZero(t, flag.CreatedAt)
	assert.NotZero(t, flag.UpdatedAt)
}

func (s *DBTestSuite) TestGetFlagNamespace() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		NamespaceKey: s.namespace,
		Key:          t.Name(),
		Name:         "foo",
		Description:  "bar",
		Enabled:      true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag)

	got, err := s.store.GetFlag(context.TODO(), s.namespace, flag.Key)

	require.NoError(t, err)
	assert.NotNil(t, got)

	assert.Equal(t, s.namespace, got.NamespaceKey)
	assert.Equal(t, flag.Key, got.Key)
	assert.Equal(t, flag.Name, got.Name)
	assert.Equal(t, flag.Description, got.Description)
	assert.Equal(t, flag.Enabled, got.Enabled)
	assert.NotZero(t, flag.CreatedAt)
	assert.NotZero(t, flag.UpdatedAt)
}

func (s *DBTestSuite) TestGetFlag_NotFound() {
	t := s.T()

	_, err := s.store.GetFlag(context.TODO(), storage.DefaultNamespace, "foo")
	assert.EqualError(t, err, "flag \"default/foo\" not found")
}

func (s *DBTestSuite) TestGetFlagNamespace_NotFound() {
	t := s.T()

	_, err := s.store.GetFlag(context.TODO(), s.namespace, "foo")
	assert.EqualError(t, err, fmt.Sprintf("flag \"%s/foo\" not found", s.namespace))
}

func (s *DBTestSuite) TestListFlags() {
	t := s.T()

	reqs := []*flipt.CreateFlagRequest{
		{
			Key:         uuid.Must(uuid.NewV4()).String(),
			Name:        "foo",
			Description: "bar",
			Enabled:     true,
		},
		{
			Key:         uuid.Must(uuid.NewV4()).String(),
			Name:        "foo",
			Description: "bar",
		},
	}

	for _, req := range reqs {
		_, err := s.store.CreateFlag(context.TODO(), req)
		require.NoError(t, err)
	}

	res, err := s.store.ListFlags(context.TODO(), storage.DefaultNamespace)
	require.NoError(t, err)

	got := res.Results
	assert.NotZero(t, len(got))

	for _, flag := range got {
		assert.Equal(t, storage.DefaultNamespace, flag.NamespaceKey)
		assert.NotZero(t, flag.CreatedAt)
		assert.NotZero(t, flag.UpdatedAt)
	}
}

func (s *DBTestSuite) TestListFlagsNamespace() {
	t := s.T()

	reqs := []*flipt.CreateFlagRequest{
		{
			NamespaceKey: s.namespace,
			Key:          uuid.Must(uuid.NewV4()).String(),
			Name:         "foo",
			Description:  "bar",
			Enabled:      true,
		},
		{
			NamespaceKey: s.namespace,
			Key:          uuid.Must(uuid.NewV4()).String(),
			Name:         "foo",
			Description:  "bar",
		},
	}

	for _, req := range reqs {
		_, err := s.store.CreateFlag(context.TODO(), req)
		require.NoError(t, err)
	}

	res, err := s.store.ListFlags(context.TODO(), s.namespace)
	require.NoError(t, err)

	got := res.Results
	assert.NotZero(t, len(got))

	for _, flag := range got {
		assert.Equal(t, s.namespace, flag.NamespaceKey)
		assert.NotZero(t, flag.CreatedAt)
		assert.NotZero(t, flag.UpdatedAt)
	}
}

func (s *DBTestSuite) TestListFlagsPagination_LimitOffset() {
	t := s.T()

	reqs := []*flipt.CreateFlagRequest{
		{
			Key:         uuid.Must(uuid.NewV4()).String(),
			Name:        "foo",
			Description: "bar",
			Enabled:     true,
		},
		{
			Key:         uuid.Must(uuid.NewV4()).String(),
			Name:        "foo",
			Description: "bar",
		},
		{
			Key:         uuid.Must(uuid.NewV4()).String(),
			Name:        "foo",
			Description: "bar",
			Enabled:     true,
		},
	}

	for _, req := range reqs {
		if s.db.Driver == fliptsql.MySQL {
			// required for MySQL since it only s.stores timestamps to the second and not millisecond granularity
			time.Sleep(time.Second)
		}
		_, err := s.store.CreateFlag(context.TODO(), req)
		require.NoError(t, err)
	}

	oldest, middle, newest := reqs[0], reqs[1], reqs[2]

	// TODO: the ordering (DESC) is required because the default ordering is ASC and we are not clearing the DB between tests
	// get middle flag
	res, err := s.store.ListFlags(context.TODO(), storage.DefaultNamespace, storage.WithOrder(storage.OrderDesc), storage.WithLimit(1), storage.WithOffset(1))
	require.NoError(t, err)

	got := res.Results
	assert.Len(t, got, 1)

	assert.Equal(t, middle.Key, got[0].Key)

	// get first (newest) flag
	res, err = s.store.ListFlags(context.TODO(), storage.DefaultNamespace, storage.WithOrder(storage.OrderDesc), storage.WithLimit(1))
	require.NoError(t, err)

	got = res.Results
	assert.Len(t, got, 1)

	assert.Equal(t, newest.Key, got[0].Key)

	// get last (oldest) flag
	res, err = s.store.ListFlags(context.TODO(), storage.DefaultNamespace, storage.WithOrder(storage.OrderDesc), storage.WithLimit(1), storage.WithOffset(2))
	require.NoError(t, err)

	got = res.Results
	assert.Len(t, got, 1)

	assert.Equal(t, oldest.Key, got[0].Key)

	// get all flags
	res, err = s.store.ListFlags(context.TODO(), storage.DefaultNamespace, storage.WithOrder(storage.OrderDesc))
	require.NoError(t, err)

	got = res.Results

	assert.Equal(t, newest.Key, got[0].Key)
	assert.Equal(t, middle.Key, got[1].Key)
	assert.Equal(t, oldest.Key, got[2].Key)
}

func (s *DBTestSuite) TestListFlagsPagination_LimitWithNextPage() {
	t := s.T()

	reqs := []*flipt.CreateFlagRequest{
		{
			Key:         uuid.Must(uuid.NewV4()).String(),
			Name:        "foo",
			Description: "bar",
			Enabled:     true,
		},
		{
			Key:         uuid.Must(uuid.NewV4()).String(),
			Name:        "foo",
			Description: "bar",
		},
		{
			Key:         uuid.Must(uuid.NewV4()).String(),
			Name:        "foo",
			Description: "bar",
			Enabled:     true,
		},
	}

	for _, req := range reqs {
		if s.db.Driver == fliptsql.MySQL {
			// required for MySQL since it only s.stores timestamps to the second and not millisecond granularity
			time.Sleep(time.Second)
		}
		_, err := s.store.CreateFlag(context.TODO(), req)
		require.NoError(t, err)
	}

	oldest, middle, newest := reqs[0], reqs[1], reqs[2]

	// TODO: the ordering (DESC) is required because the default ordering is ASC and we are not clearing the DB between tests
	// get newest flag
	opts := []storage.QueryOption{storage.WithOrder(storage.OrderDesc), storage.WithLimit(1)}

	res, err := s.store.ListFlags(context.TODO(), storage.DefaultNamespace, opts...)
	require.NoError(t, err)

	got := res.Results
	assert.Len(t, got, 1)
	assert.Equal(t, newest.Key, got[0].Key)
	assert.NotEmpty(t, res.NextPageToken)

	pageToken := &common.PageToken{}
	err = json.Unmarshal([]byte(res.NextPageToken), pageToken)
	require.NoError(t, err)
	// next page should be the middle flag
	assert.Equal(t, middle.Key, pageToken.Key)
	assert.NotZero(t, pageToken.Offset)

	opts = append(opts, storage.WithPageToken(res.NextPageToken))

	// get middle flag
	res, err = s.store.ListFlags(context.TODO(), storage.DefaultNamespace, opts...)
	require.NoError(t, err)

	got = res.Results
	assert.Len(t, got, 1)
	assert.Equal(t, middle.Key, got[0].Key)

	err = json.Unmarshal([]byte(res.NextPageToken), pageToken)
	require.NoError(t, err)
	// next page should be the oldest flag
	assert.Equal(t, oldest.Key, pageToken.Key)
	assert.NotZero(t, pageToken.Offset)

	opts = []storage.QueryOption{storage.WithOrder(storage.OrderDesc), storage.WithLimit(1), storage.WithPageToken(res.NextPageToken)}

	// get oldest flag
	res, err = s.store.ListFlags(context.TODO(), storage.DefaultNamespace, opts...)
	require.NoError(t, err)

	got = res.Results
	assert.Len(t, got, 1)
	assert.Equal(t, oldest.Key, got[0].Key)

	opts = []storage.QueryOption{storage.WithOrder(storage.OrderDesc), storage.WithLimit(3)}
	// get all flags
	res, err = s.store.ListFlags(context.TODO(), storage.DefaultNamespace, opts...)
	require.NoError(t, err)

	got = res.Results
	assert.Len(t, got, 3)
	assert.Equal(t, newest.Key, got[0].Key)
	assert.Equal(t, middle.Key, got[1].Key)
	assert.Equal(t, oldest.Key, got[2].Key)
}

func (s *DBTestSuite) TestListFlagsPagination_FullWalk() {
	t := s.T()

	namespace := uuid.Must(uuid.NewV4()).String()

	ctx := context.Background()
	_, err := s.store.CreateNamespace(ctx, &flipt.CreateNamespaceRequest{
		Key: namespace,
	})
	require.NoError(t, err)

	var (
		totalFlags = 9
		pageSize   = uint64(3)
	)

	for i := 0; i < totalFlags; i++ {
		req := flipt.CreateFlagRequest{
			NamespaceKey: namespace,
			Key:          fmt.Sprintf("flag_%03d", i),
			Name:         "foo",
			Description:  "bar",
		}

		_, err := s.store.CreateFlag(ctx, &req)
		require.NoError(t, err)

		for i := 0; i < 2; i++ {
			if i > 0 && s.db.Driver == fliptsql.MySQL {
				// required for MySQL since it only s.stores timestamps to the second and not millisecond granularity
				time.Sleep(time.Second)
			}

			_, err := s.store.CreateVariant(ctx, &flipt.CreateVariantRequest{
				NamespaceKey: namespace,
				FlagKey:      req.Key,
				Key:          fmt.Sprintf("variant_%d", i),
			})
			require.NoError(t, err)
		}
	}

	resp, err := s.store.ListFlags(ctx, namespace,
		storage.WithLimit(pageSize))
	require.NoError(t, err)

	found := resp.Results
	for token := resp.NextPageToken; token != ""; token = resp.NextPageToken {
		resp, err = s.store.ListFlags(ctx, namespace,
			storage.WithLimit(pageSize),
			storage.WithPageToken(token),
		)
		require.NoError(t, err)

		found = append(found, resp.Results...)
	}

	require.Len(t, found, totalFlags)

	for i := 0; i < totalFlags; i++ {
		assert.Equal(t, namespace, found[i].NamespaceKey)

		expectedFlag := fmt.Sprintf("flag_%03d", i)
		assert.Equal(t, expectedFlag, found[i].Key)
		assert.Equal(t, "foo", found[i].Name)
		assert.Equal(t, "bar", found[i].Description)

		require.Len(t, found[i].Variants, 2)
		assert.Equal(t, namespace, found[i].Variants[0].NamespaceKey)
		assert.Equal(t, expectedFlag, found[i].Variants[0].FlagKey)
		assert.Equal(t, "variant_0", found[i].Variants[0].Key)

		assert.Equal(t, namespace, found[i].Variants[1].NamespaceKey)
		assert.Equal(t, expectedFlag, found[i].Variants[1].FlagKey)
		assert.Equal(t, "variant_1", found[i].Variants[1].Key)
	}
}

func (s *DBTestSuite) TestCreateFlag() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		Key:         t.Name(),
		Name:        "foo",
		Description: "bar",
		Enabled:     true,
	})

	require.NoError(t, err)

	assert.Equal(t, storage.DefaultNamespace, flag.NamespaceKey)
	assert.Equal(t, t.Name(), flag.Key)
	assert.Equal(t, "foo", flag.Name)
	assert.Equal(t, "bar", flag.Description)
	assert.True(t, flag.Enabled)
	assert.NotZero(t, flag.CreatedAt)
	assert.Equal(t, flag.CreatedAt.Seconds, flag.UpdatedAt.Seconds)
}

func (s *DBTestSuite) TestCreateFlagNamespace() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		NamespaceKey: s.namespace,
		Key:          t.Name(),
		Name:         "foo",
		Description:  "bar",
		Enabled:      true,
	})

	require.NoError(t, err)

	assert.Equal(t, s.namespace, flag.NamespaceKey)
	assert.Equal(t, t.Name(), flag.Key)
	assert.Equal(t, "foo", flag.Name)
	assert.Equal(t, "bar", flag.Description)
	assert.True(t, flag.Enabled)
	assert.NotZero(t, flag.CreatedAt)
	assert.Equal(t, flag.CreatedAt.Seconds, flag.UpdatedAt.Seconds)
}

func (s *DBTestSuite) TestCreateFlag_DuplicateKey() {
	t := s.T()

	_, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		Key:         t.Name(),
		Name:        "foo",
		Description: "bar",
		Enabled:     true,
	})

	require.NoError(t, err)

	_, err = s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		Key:         t.Name(),
		Name:        "foo",
		Description: "bar",
		Enabled:     true,
	})

	assert.EqualError(t, err, "flag \"default/TestDBTestSuite/TestCreateFlag_DuplicateKey\" is not unique")
}

func (s *DBTestSuite) TestCreateFlagNamespace_DuplicateKey() {
	t := s.T()

	_, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		NamespaceKey: s.namespace,
		Key:          t.Name(),
		Name:         "foo",
		Description:  "bar",
		Enabled:      true,
	})

	require.NoError(t, err)

	_, err = s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		NamespaceKey: s.namespace,
		Key:          t.Name(),
		Name:         "foo",
		Description:  "bar",
		Enabled:      true,
	})

	assert.EqualError(t, err, fmt.Sprintf("flag \"%s/%s\" is not unique", s.namespace, t.Name()))
}

func (s *DBTestSuite) TestUpdateFlag() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		Key:         t.Name(),
		Name:        "foo",
		Description: "bar",
		Enabled:     true,
	})

	require.NoError(t, err)

	assert.Equal(t, storage.DefaultNamespace, flag.NamespaceKey)
	assert.Equal(t, t.Name(), flag.Key)
	assert.Equal(t, "foo", flag.Name)
	assert.Equal(t, "bar", flag.Description)
	assert.True(t, flag.Enabled)
	assert.NotZero(t, flag.CreatedAt)
	assert.Equal(t, flag.CreatedAt.Seconds, flag.UpdatedAt.Seconds)

	updated, err := s.store.UpdateFlag(context.TODO(), &flipt.UpdateFlagRequest{
		Key:         flag.Key,
		Name:        flag.Name,
		Description: "foobar",
		Enabled:     true,
	})

	require.NoError(t, err)

	assert.Equal(t, storage.DefaultNamespace, updated.NamespaceKey)
	assert.Equal(t, flag.Key, updated.Key)
	assert.Equal(t, flag.Name, updated.Name)
	assert.Equal(t, "foobar", updated.Description)
	assert.True(t, flag.Enabled)
	assert.NotZero(t, updated.CreatedAt)
	assert.NotZero(t, updated.UpdatedAt)
}

func (s *DBTestSuite) TestUpdateFlagNamespace() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		NamespaceKey: s.namespace,
		Key:          t.Name(),
		Name:         "foo",
		Description:  "bar",
		Enabled:      true,
	})

	require.NoError(t, err)

	assert.Equal(t, s.namespace, flag.NamespaceKey)
	assert.Equal(t, t.Name(), flag.Key)
	assert.Equal(t, "foo", flag.Name)
	assert.Equal(t, "bar", flag.Description)
	assert.True(t, flag.Enabled)
	assert.NotZero(t, flag.CreatedAt)
	assert.Equal(t, flag.CreatedAt.Seconds, flag.UpdatedAt.Seconds)

	updated, err := s.store.UpdateFlag(context.TODO(), &flipt.UpdateFlagRequest{
		NamespaceKey: s.namespace,
		Key:          flag.Key,
		Name:         flag.Name,
		Description:  "foobar",
		Enabled:      true,
	})

	require.NoError(t, err)

	assert.Equal(t, s.namespace, updated.NamespaceKey)
	assert.Equal(t, flag.Key, updated.Key)
	assert.Equal(t, flag.Name, updated.Name)
	assert.Equal(t, "foobar", updated.Description)
	assert.True(t, flag.Enabled)
	assert.NotZero(t, updated.CreatedAt)
	assert.NotZero(t, updated.UpdatedAt)
}

func (s *DBTestSuite) TestUpdateFlag_NotFound() {
	t := s.T()

	_, err := s.store.UpdateFlag(context.TODO(), &flipt.UpdateFlagRequest{
		Key:         "foo",
		Name:        "foo",
		Description: "bar",
		Enabled:     true,
	})

	assert.EqualError(t, err, "flag \"default/foo\" not found")
}

func (s *DBTestSuite) TestUpdateFlagNamespace_NotFound() {
	t := s.T()

	_, err := s.store.UpdateFlag(context.TODO(), &flipt.UpdateFlagRequest{
		NamespaceKey: s.namespace,
		Key:          "foo",
		Name:         "foo",
		Description:  "bar",
		Enabled:      true,
	})

	assert.EqualError(t, err, fmt.Sprintf("flag \"%s/foo\" not found", s.namespace))
}

func (s *DBTestSuite) TestDeleteFlag() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		Key:         t.Name(),
		Name:        "foo",
		Description: "bar",
		Enabled:     true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag)

	err = s.store.DeleteFlag(context.TODO(), &flipt.DeleteFlagRequest{Key: flag.Key})
	require.NoError(t, err)
}

func (s *DBTestSuite) TestDeleteFlagNamespace() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		NamespaceKey: s.namespace,
		Key:          t.Name(),
		Name:         "foo",
		Description:  "bar",
		Enabled:      true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag)

	err = s.store.DeleteFlag(context.TODO(), &flipt.DeleteFlagRequest{
		NamespaceKey: s.namespace,
		Key:          flag.Key,
	})

	require.NoError(t, err)
}

func (s *DBTestSuite) TestDeleteFlag_NotFound() {
	t := s.T()

	err := s.store.DeleteFlag(context.TODO(), &flipt.DeleteFlagRequest{Key: "foo"})
	require.NoError(t, err)
}

func (s *DBTestSuite) TestDeleteFlagNamespace_NotFound() {
	t := s.T()

	err := s.store.DeleteFlag(context.TODO(), &flipt.DeleteFlagRequest{
		NamespaceKey: s.namespace,
		Key:          "foo",
	})

	require.NoError(t, err)
}

func (s *DBTestSuite) TestCreateVariant() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		Key:         t.Name(),
		Name:        "foo",
		Description: "bar",
		Enabled:     true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag)

	attachment := `{"key":"value"}`
	variant, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		FlagKey:     flag.Key,
		Key:         t.Name(),
		Name:        "foo",
		Description: "bar",
		Attachment:  attachment,
	})

	require.NoError(t, err)
	assert.NotNil(t, variant)

	assert.NotZero(t, variant.Id)
	assert.Equal(t, storage.DefaultNamespace, variant.NamespaceKey)
	assert.Equal(t, flag.Key, variant.FlagKey)
	assert.Equal(t, t.Name(), variant.Key)
	assert.Equal(t, "foo", variant.Name)
	assert.Equal(t, "bar", variant.Description)
	assert.Equal(t, attachment, variant.Attachment)
	assert.NotZero(t, variant.CreatedAt)
	assert.Equal(t, variant.CreatedAt.Seconds, variant.UpdatedAt.Seconds)

	// get the flag again
	flag, err = s.store.GetFlag(context.TODO(), storage.DefaultNamespace, flag.Key)

	require.NoError(t, err)
	assert.NotNil(t, flag)

	assert.Len(t, flag.Variants, 1)
}

func (s *DBTestSuite) TestCreateVariantNamespace() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		NamespaceKey: s.namespace,
		Key:          t.Name(),
		Name:         "foo",
		Description:  "bar",
		Enabled:      true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag)

	attachment := `{"key":"value"}`
	variant, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		NamespaceKey: s.namespace,
		FlagKey:      flag.Key,
		Key:          t.Name(),
		Name:         "foo",
		Description:  "bar",
		Attachment:   attachment,
	})

	require.NoError(t, err)
	assert.NotNil(t, variant)

	assert.NotZero(t, variant.Id)
	assert.Equal(t, s.namespace, variant.NamespaceKey)
	assert.Equal(t, flag.Key, variant.FlagKey)
	assert.Equal(t, t.Name(), variant.Key)
	assert.Equal(t, "foo", variant.Name)
	assert.Equal(t, "bar", variant.Description)
	assert.Equal(t, attachment, variant.Attachment)
	assert.NotZero(t, variant.CreatedAt)
	assert.Equal(t, variant.CreatedAt.Seconds, variant.UpdatedAt.Seconds)

	// get the flag again
	flag, err = s.store.GetFlag(context.TODO(), s.namespace, flag.Key)

	require.NoError(t, err)
	assert.NotNil(t, flag)

	assert.Len(t, flag.Variants, 1)
}

func (s *DBTestSuite) TestCreateVariant_FlagNotFound() {
	t := s.T()

	_, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		FlagKey:     "foo",
		Key:         t.Name(),
		Name:        "foo",
		Description: "bar",
	})

	assert.EqualError(t, err, "flag \"default/foo\" not found")
}

func (s *DBTestSuite) TestCreateVariantNamespace_FlagNotFound() {
	t := s.T()

	_, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		NamespaceKey: s.namespace,
		FlagKey:      "foo",
		Key:          t.Name(),
		Name:         "foo",
		Description:  "bar",
	})

	assert.EqualError(t, err, fmt.Sprintf("flag \"%s/foo\" not found", s.namespace))
}

func (s *DBTestSuite) TestCreateVariant_DuplicateKey() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		Key:         t.Name(),
		Name:        "foo",
		Description: "bar",
		Enabled:     true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag)

	variant, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		FlagKey:     flag.Key,
		Key:         "foo",
		Name:        "foo",
		Description: "bar",
	})

	require.NoError(t, err)
	assert.NotNil(t, variant)

	// try to create another variant with the same name for this flag
	_, err = s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		FlagKey:     flag.Key,
		Key:         "foo",
		Name:        "foo",
		Description: "bar",
	})

	assert.EqualError(t, err, "variant \"foo\" is not unique for flag \"default/TestDBTestSuite/TestCreateVariant_DuplicateKey\"")
}

func (s *DBTestSuite) TestCreateVariantNamespace_DuplicateKey() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		NamespaceKey: s.namespace,
		Key:          t.Name(),
		Name:         "foo",
		Description:  "bar",
		Enabled:      true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag)

	variant, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		NamespaceKey: s.namespace,
		FlagKey:      flag.Key,
		Key:          "foo",
		Name:         "foo",
		Description:  "bar",
	})

	require.NoError(t, err)
	assert.NotNil(t, variant)

	// try to create another variant with the same name for this flag
	_, err = s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		NamespaceKey: s.namespace,
		FlagKey:      flag.Key,
		Key:          "foo",
		Name:         "foo",
		Description:  "bar",
	})

	assert.EqualError(t, err, fmt.Sprintf("variant \"foo\" is not unique for flag \"%s/%s\"", s.namespace, t.Name()))
}

func (s *DBTestSuite) TestCreateVariant_DuplicateKey_DifferentFlag() {
	t := s.T()

	flag1, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		Key:         fmt.Sprintf("%s_1", t.Name()),
		Name:        "foo_1",
		Description: "bar",
		Enabled:     true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag1)

	variant1, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		FlagKey:     flag1.Key,
		Key:         "foo",
		Name:        "foo",
		Description: "bar",
	})

	require.NoError(t, err)
	assert.NotNil(t, variant1)

	assert.NotZero(t, variant1.Id)
	assert.Equal(t, flag1.Key, variant1.FlagKey)
	assert.Equal(t, "foo", variant1.Key)

	flag2, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		Key:         fmt.Sprintf("%s_2", t.Name()),
		Name:        "foo_2",
		Description: "bar",
		Enabled:     true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag2)

	variant2, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		FlagKey:     flag2.Key,
		Key:         "foo",
		Name:        "foo",
		Description: "bar",
	})

	require.NoError(t, err)
	assert.NotNil(t, variant2)

	assert.NotZero(t, variant2.Id)
	assert.Equal(t, flag2.Key, variant2.FlagKey)
	assert.Equal(t, "foo", variant2.Key)
}

func (s *DBTestSuite) TestCreateVariantNamespace_DuplicateFlag_DuplicateKey() {
	t := s.T()

	flag1, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		NamespaceKey: s.namespace,
		Key:          t.Name(),
		Name:         "foo",
		Description:  "bar",
		Enabled:      true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag1)

	variant1, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		NamespaceKey: s.namespace,
		FlagKey:      flag1.Key,
		Key:          "foo",
		Name:         "foo",
		Description:  "bar",
	})

	require.NoError(t, err)
	assert.NotNil(t, variant1)

	assert.NotZero(t, variant1.Id)
	assert.Equal(t, s.namespace, variant1.NamespaceKey)
	assert.Equal(t, flag1.Key, variant1.FlagKey)
	assert.Equal(t, "foo", variant1.Key)

	flag2, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		Key:         t.Name(),
		Name:        "foo",
		Description: "bar",
		Enabled:     true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag2)

	variant2, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		FlagKey:     flag2.Key,
		Key:         "foo",
		Name:        "foo",
		Description: "bar",
	})

	require.NoError(t, err)
	assert.NotNil(t, variant2)

	assert.NotZero(t, variant2.Id)
	assert.Equal(t, storage.DefaultNamespace, variant2.NamespaceKey)
	assert.Equal(t, flag2.Key, variant2.FlagKey)
	assert.Equal(t, "foo", variant2.Key)
}

func (s *DBTestSuite) TestUpdateVariant() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		Key:         t.Name(),
		Name:        "foo",
		Description: "bar",
		Enabled:     true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag)

	attachment1 := `{"key":"value1"}`
	variant, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		FlagKey:     flag.Key,
		Key:         "foo",
		Name:        "foo",
		Description: "bar",
		Attachment:  attachment1,
	})

	require.NoError(t, err)
	assert.NotNil(t, variant)

	assert.NotZero(t, variant.Id)
	assert.Equal(t, storage.DefaultNamespace, variant.NamespaceKey)
	assert.Equal(t, flag.Key, variant.FlagKey)
	assert.Equal(t, "foo", variant.Key)
	assert.Equal(t, "foo", variant.Name)
	assert.Equal(t, "bar", variant.Description)
	assert.Equal(t, attachment1, variant.Attachment)
	assert.NotZero(t, variant.CreatedAt)
	assert.Equal(t, variant.CreatedAt.Seconds, variant.UpdatedAt.Seconds)

	updated, err := s.store.UpdateVariant(context.TODO(), &flipt.UpdateVariantRequest{
		Id:          variant.Id,
		FlagKey:     variant.FlagKey,
		Key:         variant.Key,
		Name:        variant.Name,
		Description: "foobar",
		Attachment:  `{"key":      "value2"}`,
	})

	require.NoError(t, err)

	assert.Equal(t, variant.Id, updated.Id)
	assert.Equal(t, storage.DefaultNamespace, updated.NamespaceKey)
	assert.Equal(t, variant.FlagKey, updated.FlagKey)
	assert.Equal(t, variant.Key, updated.Key)
	assert.Equal(t, variant.Name, updated.Name)
	assert.Equal(t, "foobar", updated.Description)
	assert.Equal(t, `{"key":"value2"}`, updated.Attachment)
	assert.NotZero(t, updated.CreatedAt)
	assert.NotZero(t, updated.UpdatedAt)

	// get the flag again
	flag, err = s.store.GetFlag(context.TODO(), storage.DefaultNamespace, flag.Key)

	require.NoError(t, err)
	assert.NotNil(t, flag)

	assert.Len(t, flag.Variants, 1)
}

func (s *DBTestSuite) TestUpdateVariantNamespace() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		NamespaceKey: s.namespace,
		Key:          t.Name(),
		Name:         "foo",
		Description:  "bar",
		Enabled:      true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag)

	attachment1 := `{"key":"value1"}`
	variant, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		NamespaceKey: s.namespace,
		FlagKey:      flag.Key,
		Key:          "foo",
		Name:         "foo",
		Description:  "bar",
		Attachment:   attachment1,
	})

	require.NoError(t, err)
	assert.NotNil(t, variant)

	assert.NotZero(t, variant.Id)
	assert.Equal(t, s.namespace, variant.NamespaceKey)
	assert.Equal(t, flag.Key, variant.FlagKey)
	assert.Equal(t, "foo", variant.Key)
	assert.Equal(t, "foo", variant.Name)
	assert.Equal(t, "bar", variant.Description)
	assert.Equal(t, attachment1, variant.Attachment)
	assert.NotZero(t, variant.CreatedAt)
	assert.Equal(t, variant.CreatedAt.Seconds, variant.UpdatedAt.Seconds)

	updated, err := s.store.UpdateVariant(context.TODO(), &flipt.UpdateVariantRequest{
		NamespaceKey: s.namespace,
		Id:           variant.Id,
		FlagKey:      variant.FlagKey,
		Key:          variant.Key,
		Name:         variant.Name,
		Description:  "foobar",
		Attachment:   `{"key":      "value2"}`,
	})

	require.NoError(t, err)

	assert.Equal(t, variant.Id, updated.Id)
	assert.Equal(t, s.namespace, updated.NamespaceKey)
	assert.Equal(t, variant.FlagKey, updated.FlagKey)
	assert.Equal(t, variant.Key, updated.Key)
	assert.Equal(t, variant.Name, updated.Name)
	assert.Equal(t, "foobar", updated.Description)
	assert.Equal(t, `{"key":"value2"}`, updated.Attachment)
	assert.NotZero(t, updated.CreatedAt)
	assert.NotZero(t, updated.UpdatedAt)

	// get the flag again
	flag, err = s.store.GetFlag(context.TODO(), s.namespace, flag.Key)

	require.NoError(t, err)
	assert.NotNil(t, flag)

	assert.Len(t, flag.Variants, 1)
}

func (s *DBTestSuite) TestUpdateVariant_NotFound() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		Key:         t.Name(),
		Name:        "foo",
		Description: "bar",
		Enabled:     true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag)

	_, err = s.store.UpdateVariant(context.TODO(), &flipt.UpdateVariantRequest{
		Id:          "foo",
		FlagKey:     flag.Key,
		Key:         "foo",
		Name:        "foo",
		Description: "bar",
	})

	assert.EqualError(t, err, "variant \"foo\" not found")
}

func (s *DBTestSuite) TestUpdateVariantNamespace_NotFound() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		NamespaceKey: s.namespace,
		Key:          t.Name(),
		Name:         "foo",
		Description:  "bar",
		Enabled:      true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag)

	_, err = s.store.UpdateVariant(context.TODO(), &flipt.UpdateVariantRequest{
		NamespaceKey: s.namespace,
		Id:           "foo",
		FlagKey:      flag.Key,
		Key:          "foo",
		Name:         "foo",
		Description:  "bar",
	})

	assert.EqualError(t, err, "variant \"foo\" not found")
}

func (s *DBTestSuite) TestUpdateVariant_DuplicateKey() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		Key:         t.Name(),
		Name:        "foo",
		Description: "bar",
		Enabled:     true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag)

	variant1, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		FlagKey:     flag.Key,
		Key:         "foo",
		Name:        "foo",
		Description: "bar",
	})

	require.NoError(t, err)
	assert.NotNil(t, variant1)

	variant2, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		FlagKey:     flag.Key,
		Key:         "bar",
		Name:        "bar",
		Description: "baz",
	})

	require.NoError(t, err)
	assert.NotNil(t, variant2)

	_, err = s.store.UpdateVariant(context.TODO(), &flipt.UpdateVariantRequest{
		Id:          variant2.Id,
		FlagKey:     variant2.FlagKey,
		Key:         variant1.Key,
		Name:        variant2.Name,
		Description: "foobar",
	})

	assert.EqualError(t, err, "variant \"foo\" is not unique for flag \"default/TestDBTestSuite/TestUpdateVariant_DuplicateKey\"")
}

func (s *DBTestSuite) TestUpdateVariantNamespace_DuplicateKey() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		NamespaceKey: s.namespace,
		Key:          t.Name(),
		Name:         "foo",
		Description:  "bar",
		Enabled:      true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag)

	variant1, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		NamespaceKey: s.namespace,
		FlagKey:      flag.Key,
		Key:          "foo",
		Name:         "foo",
		Description:  "bar",
	})

	require.NoError(t, err)
	assert.NotNil(t, variant1)

	variant2, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		NamespaceKey: s.namespace,
		FlagKey:      flag.Key,
		Key:          "bar",
		Name:         "bar",
		Description:  "baz",
	})

	require.NoError(t, err)
	assert.NotNil(t, variant2)

	_, err = s.store.UpdateVariant(context.TODO(), &flipt.UpdateVariantRequest{
		NamespaceKey: s.namespace,
		Id:           variant2.Id,
		FlagKey:      variant2.FlagKey,
		Key:          variant1.Key,
		Name:         variant2.Name,
		Description:  "foobar",
	})

	assert.EqualError(t, err, fmt.Sprintf("variant \"foo\" is not unique for flag \"%s/%s\"", s.namespace, t.Name()))
}

func (s *DBTestSuite) TestDeleteVariant() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		Key:         t.Name(),
		Name:        "foo",
		Description: "bar",
		Enabled:     true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag)

	variant, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		FlagKey:     flag.Key,
		Key:         "foo",
		Name:        "foo",
		Description: "bar",
	})

	require.NoError(t, err)
	assert.NotNil(t, variant)

	err = s.store.DeleteVariant(context.TODO(), &flipt.DeleteVariantRequest{FlagKey: variant.FlagKey, Id: variant.Id})
	require.NoError(t, err)

	// get the flag again
	flag, err = s.store.GetFlag(context.TODO(), storage.DefaultNamespace, flag.Key)

	require.NoError(t, err)
	assert.NotNil(t, flag)

	assert.Empty(t, flag.Variants)
}

func (s *DBTestSuite) TestDeleteVariantNamespace() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		NamespaceKey: s.namespace,
		Key:          t.Name(),
		Name:         "foo",
		Description:  "bar",
		Enabled:      true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag)

	variant, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		NamespaceKey: s.namespace,
		FlagKey:      flag.Key,
		Key:          "foo",
		Name:         "foo",
		Description:  "bar",
	})

	require.NoError(t, err)
	assert.NotNil(t, variant)

	err = s.store.DeleteVariant(context.TODO(), &flipt.DeleteVariantRequest{
		NamespaceKey: s.namespace,
		FlagKey:      variant.FlagKey,
		Id:           variant.Id,
	})
	require.NoError(t, err)

	// get the flag again
	flag, err = s.store.GetFlag(context.TODO(), s.namespace, flag.Key)

	require.NoError(t, err)
	assert.NotNil(t, flag)

	assert.Empty(t, flag.Variants)
}

func (s *DBTestSuite) TestDeleteVariant_ExistingRule() {
	t := s.T()

	// TODO
	t.SkipNow()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		Key:         t.Name(),
		Name:        "foo",
		Description: "bar",
		Enabled:     true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag)

	variant, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
		FlagKey:     flag.Key,
		Key:         "foo",
		Name:        "foo",
		Description: "bar",
	})

	require.NoError(t, err)
	assert.NotNil(t, variant)

	segment, err := s.store.CreateSegment(context.TODO(), &flipt.CreateSegmentRequest{
		Key:         t.Name(),
		Name:        "foo",
		Description: "bar",
	})

	require.NoError(t, err)
	assert.NotNil(t, segment)

	rule, err := s.store.CreateRule(context.TODO(), &flipt.CreateRuleRequest{
		FlagKey:    flag.Key,
		SegmentKey: segment.Key,
		Rank:       1,
	})

	require.NoError(t, err)
	assert.NotNil(t, rule)

	// try to delete variant with attached rule
	err = s.store.DeleteVariant(context.TODO(), &flipt.DeleteVariantRequest{
		Id:      variant.Id,
		FlagKey: flag.Key,
	})

	assert.EqualError(t, err, "atleast one rule exists that includes this variant")

	// delete the rule, then try to delete the variant again
	err = s.store.DeleteRule(context.TODO(), &flipt.DeleteRuleRequest{
		Id:      rule.Id,
		FlagKey: flag.Key,
	})

	require.NoError(t, err)

	err = s.store.DeleteVariant(context.TODO(), &flipt.DeleteVariantRequest{
		Id:      variant.Id,
		FlagKey: flag.Key,
	})

	require.NoError(t, err)
}

func (s *DBTestSuite) TestDeleteVariant_NotFound() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		Key:         t.Name(),
		Name:        "foo",
		Description: "bar",
		Enabled:     true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag)

	err = s.store.DeleteVariant(context.TODO(), &flipt.DeleteVariantRequest{
		Id:      "foo",
		FlagKey: flag.Key,
	})

	require.NoError(t, err)
}

func (s *DBTestSuite) TestDeleteVariantNamespace_NotFound() {
	t := s.T()

	flag, err := s.store.CreateFlag(context.TODO(), &flipt.CreateFlagRequest{
		NamespaceKey: s.namespace,
		Key:          t.Name(),
		Name:         "foo",
		Description:  "bar",
		Enabled:      true,
	})

	require.NoError(t, err)
	assert.NotNil(t, flag)

	err = s.store.DeleteVariant(context.TODO(), &flipt.DeleteVariantRequest{
		NamespaceKey: s.namespace,
		Id:           "foo",
		FlagKey:      flag.Key,
	})

	require.NoError(t, err)
}

func BenchmarkListFlags(b *testing.B) {
	s := new(DBTestSuite)
	t := &testing.T{}
	s.SetT(t)
	s.SetupSuite()

	for i := 0; i < 1000; i++ {
		reqs := []*flipt.CreateFlagRequest{
			{
				Key:     uuid.Must(uuid.NewV4()).String(),
				Name:    fmt.Sprintf("foo_%d", i),
				Enabled: true,
			},
		}

		for _, req := range reqs {
			f, err := s.store.CreateFlag(context.TODO(), req)
			require.NoError(t, err)
			assert.NotNil(t, f)

			for j := 0; j < 10; j++ {
				v, err := s.store.CreateVariant(context.TODO(), &flipt.CreateVariantRequest{
					FlagKey: f.Key,
					Key:     uuid.Must(uuid.NewV4()).String(),
					Name:    fmt.Sprintf("variant_%d", j),
				})

				require.NoError(t, err)
				assert.NotNil(t, v)
			}
		}
	}

	b.ResetTimer()

	b.Run("no-pagination", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			flags, err := s.store.ListFlags(context.TODO(), storage.DefaultNamespace)
			require.NoError(t, err)
			assert.NotEmpty(t, flags)
		}
	})

	for _, pageSize := range []uint64{10, 25, 100, 500} {
		pageSize := pageSize
		b.Run(fmt.Sprintf("pagination-limit-%d", pageSize), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				flags, err := s.store.ListFlags(context.TODO(), storage.DefaultNamespace, storage.WithLimit(pageSize))
				require.NoError(t, err)
				assert.NotEmpty(t, flags)
			}
		})
	}

	b.Run("pagination", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			flags, err := s.store.ListFlags(context.TODO(), storage.DefaultNamespace, storage.WithLimit(500), storage.WithOffset(50), storage.WithOrder(storage.OrderDesc))
			require.NoError(t, err)
			assert.NotEmpty(t, flags)
		}
	})

	s.TearDownSuite()
}
