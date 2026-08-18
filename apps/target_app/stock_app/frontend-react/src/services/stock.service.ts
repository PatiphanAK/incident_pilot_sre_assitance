import { apiClient } from "./api";

export const stockService = {
    // 1. GET /api/v1/stock/products
    listProducts: async () => {
        return await apiClient.get('/api/v1/stock/products');
    },

    // 2. GET /api/v1/stock/products/{id}
    getProduct: async (id: string) => {
        return await apiClient.get(`/api/v1/stock/products/${id}`);
    },

    // 3. POST /api/v1/stock/products
    createProduct: async (sku: string, name: string, description: string, price: number, quantity: number) => {
        const payload = { sku, name, description, price, quantity };
        return await apiClient.post('/api/v1/stock/products', payload);
    },

    // 4. PATCH /api/v1/stock/products/{id}
    updateProduct: async (id: string, name: string, description: string, price: number) => {
        const payload = { name, description, price };
        return await apiClient.patch(`/api/v1/stock/products/${id}`, payload);
    },

    // 5. DELETE /api/v1/stock/products/{id}
    deleteProduct: async (id: string) => {
        return await apiClient.delete(`/api/v1/stock/products/${id}`);
    },

    // 6. PATCH /api/v1/stock/products/{id}/inventory (Set absolute stock)
    adjustInventory: async (id: string, currentQty: number, deltaQty: number) => {
        const finalQty = currentQty + deltaQty;
        if (finalQty < 0) {
            throw new Error("Stock cannot be negative.");
        }
        return await apiClient.patch(`/api/v1/stock/products/${id}/inventory`, { quantity: finalQty });
    },

    // 7. POST /api/v1/stock/products/{id}/reserve (Decrement quantity testing)
    reserveStock: async (id: string, quantity: number) => {
        return await apiClient.post(`/api/v1/stock/products/${id}/reserve`, { quantity });
    }
};
