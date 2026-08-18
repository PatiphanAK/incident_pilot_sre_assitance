import { useState, useEffect } from 'react'
import { authService } from './services/auth.service'
import { stockService } from './services/stock.service'
import { orderService } from './services/order.service'

export default function App() {
  // --- 1. State: ตัวแปรเก็บข้อมูล ---
  const [user, setUser] = useState<any>(null)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [errorLog, setErrorLog] = useState('')

  const [products, setProducts] = useState<any[]>([])
  const [orders, setOrders] = useState<any[]>([])

  const [orderQtys, setOrderQtys] = useState<Record<string, number>>({})

  const [createForm, setCreateForm] = useState({ sku: '', name: '', description: '', price: 0, quantity: 0 })
  const [editForm, setEditForm] = useState({ id: '', name: '', description: '', price: 0 })
  const [invForm, setInvForm] = useState({ id: '', name: '', currentQty: 0, deltaQty: 0 })

  const [deleteTarget, setDeleteTarget] = useState<{ type: 'product' | 'order', id: string, name?: string } | null>(null)
  // State ใหม่สำหรับจำเป้าหมายตอนยืนยันการสั่งซื้อ
  const [orderTarget, setOrderTarget] = useState<{ productId: string, name: string, qty: number } | null>(null)

  const [toast, setToast] = useState<{ message: string, type: 'success' | 'danger' } | null>(null)

  const showToast = (message: string, type: 'success' | 'danger' = 'success') => {
    setToast({ message, type })
    setTimeout(() => setToast(null), 3000)
  }

  // --- 2. ฟังก์ชัน Auth ---
  const handleLogin = async () => {
    try {
      setErrorLog('')
      const res = await authService.login({ email, password })
      setUser(res.user)
      fetchProducts()
      showToast('Login successful!', 'success')
    } catch (err: any) {
      setErrorLog(`Login failed: ${err.message}`)
    }
  }

  const handleLogout = () => {
    authService.logout()
    setUser(null)
  }

  // --- 3. ฟังก์ชัน Products ---
  const fetchProducts = async () => {
    try {
      let res = await stockService.listProducts()
      const data = res.data ? res.data : res
      setProducts(Array.isArray(data) ? data : [])
    } catch (err: any) {
      showToast(`Failed to load products: ${err.message}`, 'danger')
    }
  }

  const handleCreateProduct = async () => {
    try {
      await stockService.createProduct(createForm.sku, createForm.name, createForm.description, createForm.price, createForm.quantity)
      closeModal('createProductModal')
      fetchProducts()
      setCreateForm({ sku: '', name: '', description: '', price: 0, quantity: 0 })
      showToast('Product created successfully!')
    } catch (err: any) { showToast(`Error: ${err.message}`, 'danger') }
  }

  const handleEditProduct = async () => {
    try {
      await stockService.updateProduct(editForm.id, editForm.name, editForm.description, editForm.price)
      closeModal('editProductModal')
      fetchProducts()
      showToast('Product updated successfully!')
    } catch (err: any) { showToast(`Error: ${err.message}`, 'danger') }
  }

  // --- 4. ฟังก์ชันจัดการสต็อก (Inventory) ---
  const handleAdjustInventory = async () => {
    try {
      await stockService.adjustInventory(invForm.id, invForm.currentQty, invForm.deltaQty)
      closeModal('inventoryModal')
      fetchProducts()
      showToast('Stock adjusted successfully!')
    } catch (err: any) { showToast(`Error: ${err.message}`, 'danger') }
  }

  // --- 5. ฟังก์ชัน Orders ---
  const fetchOrders = async () => {
    try {
      let res = await orderService.listOrders()
      const data = res.data ? res.data : res
      setOrders(Array.isArray(data) ? data : [])
    } catch (err: any) {
      showToast(`Failed to load orders: ${err.message}`, 'danger')
    }
  }

  // ฟังก์ชันใหม่: กดยืนยันสั่งซื้อจากหน้า Modal
  const executeOrder = async () => {
    if (!orderTarget) return
    try {
      await orderService.placeOrder(orderTarget.productId, orderTarget.qty)
      showToast(`Successfully ordered ${orderTarget.qty}x ${orderTarget.name}!`)
      fetchProducts()
      closeModal('orderConfirmModal')
      setOrderTarget(null)
    } catch (err: any) { showToast(`Order failed: ${err.message}`, 'danger') }
  }

  const handleChangeOrderStatus = async (id: string, status: string) => {
    try {
      await orderService.updateOrderStatus(id, status)
      fetchOrders()
      fetchProducts()
      showToast('Order status updated!')
    } catch (err: any) { showToast(`Update failed: ${err.message}`, 'danger') }
  }

  // --- 6. ฟังก์ชันยืนยันการลบแบบรวม (Execute Delete) ---
  const executeDelete = async () => {
    if (!deleteTarget) return
    try {
      if (deleteTarget.type === 'product') {
        await stockService.deleteProduct(deleteTarget.id)
        fetchProducts()
        showToast('Product deleted successfully!')
      } else if (deleteTarget.type === 'order') {
        await orderService.deleteOrder(deleteTarget.id)
        fetchOrders()
        showToast('Order deleted successfully!')
      }
      closeModal('deleteConfirmModal')
      setDeleteTarget(null)
    } catch (err: any) {
      showToast(`Delete failed: ${err.message}`, 'danger')
    }
  }

  const closeModal = (id: string) => {
    const modalElement = document.getElementById(id)
    if (modalElement) {
      const closeButton = modalElement.querySelector('.btn-close') as HTMLElement
      if (closeButton) closeButton.click()
    }
  }

  // --- 7. UI (หน้าตาเว็บ) ---
  if (!user) {
    return (
      <div className="container mt-5 pt-5">
        <div className="row justify-content-center">
          <div className="col-md-5">
            <h2 className="text-center fw-bold text-primary mb-4">
              <i className="bi bi-box-seam me-2"></i>Stock Management System
            </h2>
            <div className="card card-shadow p-5">
              <h3 className="fw-bold text-center mb-4">Login</h3>
              <input type="email" className="form-control mb-3" placeholder="Email" value={email} onChange={e => setEmail(e.target.value)} />
              <input type="password" className="form-control mb-3" placeholder="Password" value={password} onChange={e => setPassword(e.target.value)} />
              <button className="btn btn-primary w-100 py-2" onClick={handleLogin}>Sign In</button>
              {errorLog && <div className="text-danger text-center mt-3 fw-bold">{errorLog}</div>}
            </div>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div>
      <nav className="navbar navbar-expand-lg navbar-light bg-white shadow-sm mb-4">
        <div className="container">
          {/* เปลี่ยนชื่อเป็น Stock Management System แล้วครับ */}
          <span className="navbar-brand fw-bold text-primary"><i className="bi bi-box-seam me-2"></i>Stock Management System</span>
          <div>
            <span className="me-3 text-muted fw-bold">{user.username}</span>
            <button className="btn btn-outline-danger btn-sm" onClick={handleLogout}><i className="bi bi-box-arrow-right"></i> Logout</button>
          </div>
        </div>
      </nav>

      <div className="container pb-5">
        <div className="d-flex justify-content-between align-items-center mb-4">
          <h4 className="fw-bold m-0"><i className="bi bi-list-ul me-2"></i>Product Catalog</h4>
          <div>
            <button className="btn btn-outline-primary me-2" data-bs-toggle="modal" data-bs-target="#orderHistoryModal" onClick={fetchOrders}>
              <i className="bi bi-receipt"></i> Orders
            </button>
            <button className="btn btn-primary me-2" data-bs-toggle="modal" data-bs-target="#createProductModal">
              <i className="bi bi-plus-lg"></i> Add Product
            </button>
            <button className="btn btn-success" onClick={fetchProducts}>
              <i className="bi bi-arrow-clockwise"></i> Refresh
            </button>
          </div>
        </div>

        {/* Product List */}
        <div className="row">
          {products.length === 0 ? (
            <div className="text-muted text-center py-5">No products found</div>
          ) : (
            products.map((p) => {
              const outOfStock = p.quantity <= 0
              // ป้องกันไม่ให้กด Order ถ้าเผลอพิมพ์เลข 0 หรือติดลบ
              const currentQtyInput = orderQtys[p.id] !== undefined ? orderQtys[p.id] : 1
              const isInvalidQty = currentQtyInput <= 0

              return (
                <div className="col-md-4 mb-4" key={p.id}>
                  <div className="card card-shadow product-card h-100 d-flex flex-column">
                    <button
                      className="btn btn-sm btn-link text-danger delete-btn"
                      data-bs-toggle="modal"
                      data-bs-target="#deleteConfirmModal"
                      onClick={() => setDeleteTarget({ type: 'product', id: p.id, name: p.name })}
                    >
                      <i className="bi bi-trash fs-5"></i>
                    </button>

                    <div className="card-body mt-2 d-flex flex-column">
                      <div className="d-flex justify-content-between mb-1 pe-4">
                        <span className="text-muted small">{p.sku}</span>
                        {outOfStock ? <span className="badge bg-danger">Out of Stock</span> : <span className="badge bg-success">{p.quantity} In Stock</span>}
                      </div>
                      <h5 className="fw-bold mb-1">{p.name}</h5>
                      <p className="text-muted small text-truncate mb-2">{p.description || '-'}</p>
                      <h4 className="text-primary fw-bold mb-3">${p.price.toFixed(2)}</h4>

                      <div className="input-group input-group-sm mb-3">
                        <span className="input-group-text bg-light text-muted">Qty</span>
                        <input
                          type="number"
                          className="form-control text-center"
                          value={currentQtyInput}
                          min="1"
                          max={p.quantity}
                          disabled={outOfStock}
                          onChange={(e) => setOrderQtys({...orderQtys, [p.id]: parseInt(e.target.value) || 0})}
                        />
                        {/* ปุ่มเปิด Modal สั่งซื้อ */}
                        <button
                          className="btn btn-success"
                          disabled={outOfStock || isInvalidQty}
                          data-bs-toggle="modal"
                          data-bs-target="#orderConfirmModal"
                          onClick={() => setOrderTarget({ productId: p.id, name: p.name, qty: currentQtyInput })}
                        >
                          <i className="bi bi-cart-plus me-1"></i> Order
                        </button>
                      </div>

                      <div className="row g-2 mt-auto pt-2 border-top">
                        <div className="col-6">
                          <button
                            className="btn btn-outline-secondary w-100 btn-sm py-2"
                            data-bs-toggle="modal"
                            data-bs-target="#editProductModal"
                            onClick={() => setEditForm({ id: p.id, name: p.name, description: p.description, price: p.price })}
                          >Edit Info</button>
                        </div>
                        <div className="col-6">
                          <button
                            className="btn btn-primary w-100 btn-sm py-2"
                            data-bs-toggle="modal"
                            data-bs-target="#inventoryModal"
                            onClick={() => setInvForm({ id: p.id, name: p.name, currentQty: p.quantity, deltaQty: 0 })}
                          >Adjust Stock</button>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              )
            })
          )}
        </div>
      </div>

      {/* --- Modals --- */}

      {/* Modal: ยืนยันการสั่งซื้อสินค้า (มาใหม่!) */}
      <div className="modal fade" id="orderConfirmModal" tabIndex={-1}>
        <div className="modal-dialog modal-sm">
          <div className="modal-content">
            <div className="modal-header border-0 pb-0">
              <button type="button" className="btn-close" data-bs-dismiss="modal"></button>
            </div>
            <div className="modal-body text-center pb-4">
              <i className="bi bi-cart-check text-success mb-3" style={{ fontSize: '3rem' }}></i>
              <h5 className="fw-bold">Confirm Order</h5>
              <p className="text-muted small mb-4">
                Are you sure you want to order<br/>
                <strong className="text-dark">{orderTarget?.qty}x {orderTarget?.name}</strong>?
              </p>
              <div className="d-flex justify-content-center gap-2">
                <button type="button" className="btn btn-light" data-bs-dismiss="modal">Cancel</button>
                <button type="button" className="btn btn-success" onClick={executeOrder}>Confirm Order</button>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Modal: ยืนยันการลบ (แก้ไขให้โชว์รหัสออเดอร์แทนคำว่า this Order) */}
      <div className="modal fade" id="deleteConfirmModal" tabIndex={-1}>
        <div className="modal-dialog modal-sm">
          <div className="modal-content">
            <div className="modal-header border-0 pb-0">
              <button type="button" className="btn-close" data-bs-dismiss="modal"></button>
            </div>
            <div className="modal-body text-center pb-4">
              <i className="bi bi-exclamation-circle text-danger mb-3" style={{ fontSize: '3rem' }}></i>
              <h5 className="fw-bold">Are you sure?</h5>
              <p className="text-muted small mb-4">
                You are about to delete<br/>
                {deleteTarget?.type === 'product'
                  ? <strong className="text-dark">"{deleteTarget.name}"</strong>
                  : <strong className="text-dark">Order: {deleteTarget?.id.substring(0,8)}...</strong>
                }.<br/>
                This action cannot be undone.
              </p>
              <div className="d-flex justify-content-center gap-2">
                <button type="button" className="btn btn-light" data-bs-dismiss="modal">Cancel</button>
                <button type="button" className="btn btn-danger" onClick={executeDelete}>Yes, delete it</button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="modal fade" id="createProductModal" tabIndex={-1}>
        <div className="modal-dialog">
          <div className="modal-content">
            <div className="modal-header"><h5 className="modal-title fw-bold text-primary">Create Product</h5><button type="button" className="btn-close" data-bs-dismiss="modal"></button></div>
            <div className="modal-body">
              <input type="text" className="form-control mb-3" placeholder="SKU" value={createForm.sku} onChange={e => setCreateForm({...createForm, sku: e.target.value})} />
              <input type="text" className="form-control mb-3" placeholder="Product Name" value={createForm.name} onChange={e => setCreateForm({...createForm, name: e.target.value})} />
              <textarea className="form-control mb-3" placeholder="Description" value={createForm.description} onChange={e => setCreateForm({...createForm, description: e.target.value})}></textarea>
              <div className="row">
                <div className="col-6"><input type="number" className="form-control" placeholder="Price ($)" value={createForm.price} onChange={e => setCreateForm({...createForm, price: parseFloat(e.target.value)})} /></div>
                <div className="col-6"><input type="number" className="form-control" placeholder="Initial Qty" value={createForm.quantity} onChange={e => setCreateForm({...createForm, quantity: parseInt(e.target.value)})} /></div>
              </div>
            </div>
            <div className="modal-footer bg-light"><button className="btn btn-primary" onClick={handleCreateProduct}>Save Product</button></div>
          </div>
        </div>
      </div>

      <div className="modal fade" id="editProductModal" tabIndex={-1}>
        <div className="modal-dialog">
          <div className="modal-content">
            <div className="modal-header"><h5 className="modal-title fw-bold text-warning">Edit Product Info</h5><button type="button" className="btn-close" data-bs-dismiss="modal"></button></div>
            <div className="modal-body">
              <label className="form-label small text-muted">Product Name</label>
              <input type="text" className="form-control mb-3" value={editForm.name} onChange={e => setEditForm({...editForm, name: e.target.value})} />
              <label className="form-label small text-muted">Description</label>
              <textarea className="form-control mb-3" value={editForm.description} onChange={e => setEditForm({...editForm, description: e.target.value})}></textarea>
              <label className="form-label small text-muted">Price ($)</label>
              <input type="number" className="form-control mb-3" value={editForm.price} onChange={e => setEditForm({...editForm, price: parseFloat(e.target.value)})} />
            </div>
            <div className="modal-footer bg-light"><button className="btn btn-warning text-dark" onClick={handleEditProduct}>Save Changes</button></div>
          </div>
        </div>
      </div>

      <div className="modal fade" id="inventoryModal" tabIndex={-1}>
        <div className="modal-dialog modal-sm">
          <div className="modal-content">
            <div className="modal-header"><h5 className="modal-title fw-bold text-success">Adjust Stock</h5><button type="button" className="btn-close" data-bs-dismiss="modal"></button></div>
            <div className="modal-body text-center">
              <h5 className="fw-bold mb-1">{invForm.name}</h5>
              <p className="text-muted small mb-3">Current Stock: <span className="fw-bold text-dark fs-6">{invForm.currentQty}</span></p>
              <label className="form-label small fw-bold text-primary">Adjustment (+ to add, - to reduce)</label>
              <input
                type="number"
                className="form-control text-center fs-4 mb-2"
                placeholder="e.g., 3 or -2"
                value={invForm.deltaQty || ''}
                onChange={e => setInvForm({...invForm, deltaQty: parseInt(e.target.value) || 0})}
              />
              <div className="small text-muted">New Stock will be: <span className={invForm.currentQty + invForm.deltaQty < 0 ? 'fw-bold text-danger' : 'fw-bold text-success'}>{invForm.currentQty + (invForm.deltaQty || 0)}</span></div>
            </div>
            <div className="modal-footer bg-light p-2 justify-content-center"><button className="btn btn-success w-100" onClick={handleAdjustInventory}>Update Stock</button></div>
          </div>
        </div>
      </div>

      <div className="modal fade" id="orderHistoryModal" tabIndex={-1}>
        <div className="modal-dialog modal-xl">
          <div className="modal-content">
            <div className="modal-header bg-light"><h5 className="modal-title fw-bold"><i className="bi bi-receipt me-2"></i>All Orders</h5><button type="button" className="btn-close" data-bs-dismiss="modal"></button></div>
            <div className="modal-body">
              <div className="table-responsive">
                <table className="table table-hover align-middle">
                  <thead className="table-light"><tr><th>Order ID</th><th>Date</th><th>Total</th><th>Status</th><th>Action</th></tr></thead>
                  <tbody>
                    {orders.length === 0 ? (
                      <tr><td colSpan={5} className="text-center text-muted py-4">No orders found</td></tr>
                    ) : (
                      orders.map(o => (
                        <tr key={o.id}>
                          <td className="small text-muted">{o.id.substring(0,8)}...</td>
                          <td>{new Date(o.created_at).toLocaleString('en-US')}</td>
                          <td className="fw-bold text-success">${o.total_price ? o.total_price.toFixed(2) : '0.00'}</td>
                          <td>
                            <select
                              className={`form-select form-select-sm w-auto ${o.status === 'CANCELLED' ? 'border-danger text-danger' : ''}`}
                              value={o.status}
                              disabled={o.status === 'CANCELLED' || o.status === 'SHIPPED'}
                              onChange={(e) => handleChangeOrderStatus(o.id, e.target.value)}
                            >
                              <option value="PENDING">PENDING</option>
                              <option value="PAID">PAID</option>
                              <option value="SHIPPED">SHIPPED</option>
                              <option value="CANCELLED">CANCELLED</option>
                            </select>
                          </td>
                          <td>
                            <button
                              className="btn btn-sm btn-outline-danger"
                              data-bs-toggle="modal"
                              data-bs-target="#deleteConfirmModal"
                              onClick={() => setDeleteTarget({ type: 'order', id: o.id })}
                            >
                              <i className="bi bi-trash"></i>
                            </button>
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* --- ระบบแจ้งเตือน (Toast Notification) --- */}
      {toast && (
        <div className="position-fixed bottom-0 end-0 p-3" style={{ zIndex: 1060 }}>
          <div className={`alert alert-${toast.type} shadow-lg d-flex align-items-center mb-0`} style={{ minWidth: '250px' }}>
            <i className={`bi ${toast.type === 'success' ? 'bi-check-circle-fill' : 'bi-exclamation-triangle-fill'} me-2 fs-4`}></i>
            <strong className="ms-2">{toast.message}</strong>
          </div>
        </div>
      )}

    </div>
  )
}
