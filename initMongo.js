// ======================================================
// INSERCIÓN DE DATOS REALISTAS PARA MONGODB
// ======================================================

// Cambiar a la base de datos del restaurante
db = db.getSiblingDB('restaurante_db');

// 1. Insertar roles
db.roles_usuario.insertMany([
    { _id: 1, nombre_rol: "administrador" },
    { _id: 2, nombre_rol: "cliente" }
]);

// 2. Insertar estados
db.estados_reserva.insertMany([
    { _id: 1, nombre_estado: "pendiente" },
    { _id: 2, nombre_estado: "confirmada" },
    { _id: 3, nombre_estado: "cancelada" }
]);

db.estados_pedido.insertMany([
    { _id: 1, nombre_estado: "pendiente" },
    { _id: 2, nombre_estado: "en_preparacion" },
    { _id: 3, nombre_estado: "completado" },
    { _id: 4, nombre_estado: "entregado" }
]);

// 3. Usuarios
db.usuarios.insertMany([
    { _id: "admin_sj_001", nombre: "Carlos Vargas Rodríguez", email: "carlos.vargas@lafondacr.com", keycloak_id: "kc_admin_sj_001", id_rol: 1, fecha_creacion: new Date() },
    { _id: "admin_cartago_002", nombre: "Mariana Castro Jiménez", email: "mariana.castro@antojosticos.cr", keycloak_id: "kc_admin_cartago_002", id_rol: 1, fecha_creacion: new Date() },
    { _id: "admin_heredia_003", nombre: "Randall Fallas Solano", email: "randall.fallas@umamihn.com", keycloak_id: "kc_admin_heredia_003", id_rol: 1, fecha_creacion: new Date() },
    { _id: "cliente_ejemplo_001", nombre: "Laura Fernández Segura", email: "laura.fernandez@email.com", keycloak_id: "kc_cliente_001", id_rol: 2, fecha_creacion: new Date() }
]);

// 4. Restaurantes
db.restaurantes.insertMany([
    { _id: "rest_sj_lafonda_001", nombre: "La Fonda Criolla", direccion: "Avenida Central, San José", telefono: "+506 2256-7890", id_administrador: "admin_sj_001", capacidad_total: 80, fecha_creacion: new Date() },
    { _id: "rest_cartago_antojos_002", nombre: "Antojos Ticos Fusión", direccion: "Barrio Los Ángeles, Cartago", telefono: "+506 2551-3245", id_administrador: "admin_cartago_002", capacidad_total: 60, fecha_creacion: new Date() },
    { _id: "rest_heredia_umami_003", nombre: "Umami House", direccion: "San Joaquín, Heredia", telefono: "+506 2262-9876", id_administrador: "admin_heredia_003", capacidad_total: 70, fecha_creacion: new Date() }
]);

// 5. Platos
db.platos.insertMany([
    { _id: "plato_sj_chifrijo_001", id_restaurante: "rest_sj_lafonda_001", nombre: "Chifrijo Gourmet", descripcion: "Tradicional chifrijo", precio: 6500.0, esta_disponible: true, categoria: "tipica" },
    { _id: "plato_sj_ceviche_005", id_restaurante: "rest_sj_lafonda_001", nombre: "Ceviche de Corvina al Coco", descripcion: "Corvina fresca", precio: 8500.0, esta_disponible: true, categoria: "mariscos" },
    { _id: "plato_cartago_risotto_006", id_restaurante: "rest_cartago_antojos_002", nombre: "Gallo Pinto Risotto", descripcion: "Risotto cremoso", precio: 7800.0, esta_disponible: true, categoria: "fusion" },
    { _id: "plato_heredia_ramen_011", id_restaurante: "rest_heredia_umami_003", nombre: "Ramen Tico-Ramen", descripcion: "Caldo Tonkotsu", precio: 9500.0, esta_disponible: true, categoria: "asiatica" }
]);

// 6. Reserva y Pedido
db.reservas.insertOne({ _id: "reserva_ejemplo_001", id_usuario: "cliente_ejemplo_001", id_restaurante: "rest_sj_lafonda_001", fecha_reserva: new Date(), cantidad_personas: 4, id_estado: 2, nombre_estado: "confirmada", fecha_creacion: new Date() });
db.pedidos.insertOne({ _id: "pedido_ejemplo_001", id_usuario: "cliente_ejemplo_001", id_restaurante: "rest_sj_lafonda_001", es_recogida: false, id_estado_pedido: 1, total: 21500.0, fecha_creacion: new Date(), detalles: [{ _id: "detalle_001", id_plato: "plato_sj_ceviche_005", cantidad: 1, precio: 8500.0 }, { _id: "detalle_002", id_plato: "plato_sj_chifrijo_001", cantidad: 2, precio: 6500.0 }] });

print("✅ Datos insertados exitosamente!");