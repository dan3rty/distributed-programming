package tests

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"order/pkg/domain/model"
	"order/pkg/domain/service"
)

var _ model.OrderRepository = (*mockOrderRepository)(nil)

type mockOrderRepository struct {
	store map[uuid.UUID]*model.Order
}

func (m *mockOrderRepository) NextID() (uuid.UUID, error) {
	return uuid.NewV7()
}

func (m *mockOrderRepository) Store(order *model.Order) error {
	orderCopy := *order
	itemsCopy := make([]model.Item, len(order.Items))
	copy(itemsCopy, order.Items)
	orderCopy.Items = itemsCopy
	m.store[order.ID] = &orderCopy
	return nil
}

func (m *mockOrderRepository) Find(id uuid.UUID) (*model.Order, error) {
	order, ok := m.store[id]
	if !ok || order.DeletedAt != nil {
		return nil, model.ErrOrderNotFound
	}

	orderCopy := *order
	itemsCopy := make([]model.Item, len(order.Items))
	copy(itemsCopy, order.Items)
	orderCopy.Items = itemsCopy
	return &orderCopy, nil
}

func (m *mockOrderRepository) Delete(id uuid.UUID) error {
	if order, ok := m.store[id]; ok && order.DeletedAt == nil {
		now := time.Now()
		order.DeletedAt = &now
		return nil
	}

	return model.ErrOrderNotFound
}

var _ service.EventDispatcher = (*mockEventDispatcher)(nil)

type mockEventDispatcher struct {
	events []service.Event
}

func (m *mockEventDispatcher) Dispatch(event service.Event) error {
	m.events = append(m.events, event)
	return nil
}

func (m *mockEventDispatcher) Clear() {
	m.events = nil
}

func TestOrderService_CreateOrder(t *testing.T) {
	repo := &mockOrderRepository{store: make(map[uuid.UUID]*model.Order)}
	dispatcher := &mockEventDispatcher{}
	orderService := service.NewOrderService(repo, dispatcher)

	customerID := uuid.New()

	t.Run("successfully creates an order", func(t *testing.T) {
		orderID, err := orderService.CreateOrder(customerID)

		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, orderID)

		createdOrder, findErr := repo.Find(orderID)
		require.NoError(t, findErr)
		assert.Equal(t, orderID, createdOrder.ID)
		assert.Equal(t, customerID, createdOrder.CustomerID)
		assert.Equal(t, model.Open, createdOrder.Status)
		assert.Empty(t, createdOrder.Items)

		require.Len(t, dispatcher.events, 1)
		event, ok := dispatcher.events[0].(model.OrderCreated)
		require.True(t, ok, "event should be of type OrderCreated")
		assert.Equal(t, orderID, event.OrderID)
		assert.Equal(t, customerID, event.CustomerID)
	})
}

func TestOrderService_SetStatus_Transitions(t *testing.T) {
	testCases := []struct {
		name          string
		initialStatus model.OrderStatus
		newStatus     model.OrderStatus
		expectError   bool
		expectedEvent bool
	}{
		{"From Open to Pending", model.Open, model.Pending, false, true},
		{"From Open to Cancelled", model.Open, model.Cancelled, false, true},
		{"From Pending to Paid", model.Pending, model.Paid, false, true},
		{"From Pending to Cancelled", model.Pending, model.Cancelled, false, true},
		{"From Pending back to Open", model.Pending, model.Open, false, true},
		{"From Paid to Cancelled (return)", model.Paid, model.Cancelled, false, true},

		{"From Open to Paid", model.Open, model.Paid, true, false},
		{"From Paid to Open", model.Paid, model.Open, true, false},
		{"From Paid to Pending", model.Paid, model.Pending, true, false},
		{"From Cancelled to Open", model.Cancelled, model.Open, true, false},
		{"From Cancelled to Pending", model.Cancelled, model.Pending, true, false},

		{"From Open to Open", model.Open, model.Open, false, false},
		{"From Pending to Pending", model.Pending, model.Pending, false, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mockOrderRepository{store: make(map[uuid.UUID]*model.Order)}
			dispatcher := &mockEventDispatcher{}
			orderService := service.NewOrderService(repo, dispatcher)

			orderID := uuid.New()
			repo.store[orderID] = &model.Order{ID: orderID, Status: tc.initialStatus}

			if tc.newStatus == model.Paid {
				repo.store[orderID].Items = append(repo.store[orderID].Items, model.Item{ID: uuid.New()})
			}

			err := orderService.SetStatus(orderID, tc.newStatus)

			if tc.expectError {
				require.Error(t, err, "Expected an error for this transition")
				assert.ErrorIs(t, err, service.ErrInvalidOrderStatus, "Error should be of type ErrInvalidOrderStatus")
			} else {
				require.NoError(t, err, "Did not expect an error for this transition")
			}

			_, _ = repo.Find(orderID)
			expectedStatusInRepo := tc.initialStatus
			if !tc.expectError {
				expectedStatusInRepo = tc.newStatus
			}
			assert.Equal(t, expectedStatusInRepo, repo.store[orderID].Status, "Order status in repo should be correct")

			if tc.expectedEvent {
				assert.Len(t, dispatcher.events, 1, "Expected one event to be dispatched")
			} else {
				assert.Empty(t, dispatcher.events, "Expected no events to be dispatched")
			}
		})
	}
}

func TestOrderService_ItemManagement(t *testing.T) {
	repo := &mockOrderRepository{store: make(map[uuid.UUID]*model.Order)}
	dispatcher := &mockEventDispatcher{}
	orderService := service.NewOrderService(repo, dispatcher)
	customerID := uuid.New()
	orderID, _ := orderService.CreateOrder(customerID)
	dispatcher.Clear()

	t.Run("AddItem fails for non-open order", func(t *testing.T) {
		_ = orderService.SetStatus(orderID, model.Pending)
		dispatcher.Clear()

		_, err := orderService.AddItem(orderID, uuid.New(), 50.0)
		require.Error(t, err)
		assert.ErrorIs(t, err, service.ErrInvalidOrderStatus)
		assert.Empty(t, dispatcher.events, "No event should be dispatched on failure")

		_ = orderService.SetStatus(orderID, model.Open)
		dispatcher.Clear()
	})

	t.Run("DeleteItem fails for non-existent item", func(t *testing.T) {
		nonExistentItemID := uuid.New()
		err := orderService.DeleteItem(orderID, nonExistentItemID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "item not found", "Error message should indicate item not found")
		assert.Empty(t, dispatcher.events)
	})

	t.Run("DeleteItem succeeds and removes correct item", func(t *testing.T) {
		item1ID, _ := orderService.AddItem(orderID, uuid.New(), 10.0)
		item2ID, _ := orderService.AddItem(orderID, uuid.New(), 20.0)
		dispatcher.Clear()

		err := orderService.DeleteItem(orderID, item1ID)
		require.NoError(t, err)

		order, _ := repo.Find(orderID)
		require.Len(t, order.Items, 1, "There should be one item left")
		assert.Equal(t, item2ID, order.Items[0].ID, "The remaining item should be the second one")

		require.Len(t, dispatcher.events, 1)
		event, ok := dispatcher.events[0].(model.OrderItemChanged)
		require.True(t, ok)
		assert.Equal(t, []uuid.UUID{item1ID}, event.RemovedItems)
		assert.Empty(t, event.AddedItems)
	})
}

func TestOrderService_DeleteOrder(t *testing.T) {
	repo := &mockOrderRepository{store: make(map[uuid.UUID]*model.Order)}
	dispatcher := &mockEventDispatcher{}
	orderService := service.NewOrderService(repo, dispatcher)
	customerID := uuid.New()
	orderID, _ := orderService.CreateOrder(customerID)
	dispatcher.Clear()

	t.Run("successfully deletes an order", func(t *testing.T) {
		dispatcher.Clear()
		err := orderService.DeleteOrder(orderID)
		require.NoError(t, err)

		deletedOrder := repo.store[orderID]
		require.NotNil(t, deletedOrder.DeletedAt, "DeletedAt should be set")

		_, err = repo.Find(orderID)
		assert.ErrorIs(t, err, model.ErrOrderNotFound, "Find should not return a soft-deleted order")

		require.Len(t, dispatcher.events, 1)
		event, ok := dispatcher.events[0].(model.OrderDeleted)
		require.True(t, ok)
		assert.Equal(t, orderID, event.OrderID)
	})

	t.Run("fails to delete non-existent order", func(t *testing.T) {
		dispatcher.Clear()
		err := orderService.DeleteOrder(uuid.New())
		require.Error(t, err)
		assert.ErrorIs(t, err, model.ErrOrderNotFound)
		assert.Empty(t, dispatcher.events, "No event on failure")
	})
}
