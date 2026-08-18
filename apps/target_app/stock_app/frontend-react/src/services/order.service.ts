import { apiClient } from "./api";

export const orderService = {
    // 1. GET /api/v1/orders
    listOrders: async () => {
        return await apiClient.get('/api/v1/orders');
    },

    // 2. GET /api/v1/orders/{id}
    getOrder: async (id: string) => {
        return await apiClient.get(`/api/v1/orders/${id}`);
    },

    // 3. POST /api/v1/orders
    placeOrder: async (productId: string, quantity: number) => {
        const payload = {
            items: [{ product_id: productId, quantity: quantity }]
        };
        return await apiClient.post('/api/v1/orders', payload);
    },

    // 4. PATCH /api/v1/orders/{id}
    updateOrderStatus: async (id: string, status: string) => {
        return await apiClient.patch(`/api/v1/orders/${id}`, { status });
    },

    // 5. DELETE /api/v1/orders/{id}
    deleteOrder: async (id: string) => {
        return await apiClient.delete(`/api/v1/orders/${id}`);
    }
};
