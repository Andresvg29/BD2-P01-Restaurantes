CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS roles_usuario (
    id SERIAL PRIMARY KEY,
    nombre_rol VARCHAR(20) UNIQUE NOT NULL 
);

CREATE TABLE IF NOT EXISTS estados_reserva (
    id SERIAL PRIMARY KEY,
    nombre_estado VARCHAR(20) UNIQUE NOT NULL 
);


CREATE TABLE IF NOT EXISTS estados_pedido (
    id SERIAL PRIMARY KEY,
    nombre_estado VARCHAR(20) UNIQUE NOT NULL 
);

CREATE TABLE IF NOT EXISTS usuarios (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    nombre VARCHAR(100) NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    keycloak_id VARCHAR(100) UNIQUE NOT NULL,
    id_rol INT NOT NULL,
    fecha_creacion TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_rol_usuario FOREIGN KEY (id_rol) REFERENCES roles_usuario(id)
);

CREATE TABLE IF NOT EXISTS restaurantes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    nombre VARCHAR(150) NOT NULL,
    direccion TEXT NOT NULL,
    ciudad VARCHAR(100) DEFAULT '',
    telefono VARCHAR(20),
    id_administrador UUID NOT NULL,
    capacidad_total INT NOT NULL DEFAULT 50,
    CONSTRAINT fk_admin_restaurante FOREIGN KEY (id_administrador) REFERENCES usuarios(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS platos (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    id_restaurante UUID NOT NULL,
    nombre VARCHAR(100) NOT NULL,
    descripcion TEXT,
    precio DECIMAL(10, 2) NOT NULL CHECK (precio >= 0),
    esta_disponible BOOLEAN DEFAULT TRUE,
    categoria VARCHAR(50) DEFAULT 'general',
    CONSTRAINT fk_restaurante_plato FOREIGN KEY (id_restaurante) REFERENCES restaurantes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS reservas (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    id_usuario UUID NOT NULL,
    id_restaurante UUID NOT NULL,
    fecha_reserva TIMESTAMP WITH TIME ZONE NOT NULL,
    cantidad_personas INT NOT NULL CHECK (cantidad_personas > 0),
    id_estado INT NOT NULL,
    fecha_creacion TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_usuario_reserva FOREIGN KEY (id_usuario) REFERENCES usuarios(id),
    CONSTRAINT fk_restaurante_reserva FOREIGN KEY (id_restaurante) REFERENCES restaurantes(id),
    CONSTRAINT fk_estado_reserva FOREIGN KEY (id_estado) REFERENCES estados_reserva(id)
);

CREATE TABLE IF NOT EXISTS pedidos (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    id_usuario UUID NOT NULL,
    id_restaurante UUID NOT NULL,
    id_reserva UUID,
    es_recogida BOOLEAN DEFAULT FALSE, -- true para recoger, false para pedir en la mesa
    id_estado_pedido INT NOT NULL,
    total DECIMAL(10, 2) DEFAULT 0,
    fecha_creacion TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_usuario_pedido FOREIGN KEY (id_usuario) REFERENCES usuarios(id),
    CONSTRAINT fk_restaurante_pedido FOREIGN KEY (id_restaurante) REFERENCES restaurantes(id),
    CONSTRAINT fk_reserva_pedido FOREIGN KEY (id_reserva) REFERENCES reservas(id) ON DELETE SET NULL,
    CONSTRAINT fk_estado_pedido FOREIGN KEY (id_estado_pedido) REFERENCES estados_pedido(id)
);

CREATE TABLE IF NOT EXISTS detalles_pedido (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    id_pedido UUID NOT NULL,
    id_plato UUID NOT NULL,
    cantidad INT NOT NULL CHECK (cantidad > 0),
    precio DECIMAL(10, 2) NOT NULL,
    
    CONSTRAINT fk_pedido_detalle FOREIGN KEY (id_pedido) REFERENCES pedidos(id) ON DELETE CASCADE,
    CONSTRAINT fk_plato_detalle FOREIGN KEY (id_plato) REFERENCES platos(id)
);

INSERT INTO roles_usuario (id, nombre_rol) VALUES (1, 'administrador'), (2, 'cliente') ON CONFLICT DO NOTHING;
INSERT INTO estados_reserva (nombre_estado) VALUES ('pendiente'), ('confirmada'), ('cancelada') ON CONFLICT DO NOTHING;
INSERT INTO estados_pedido (nombre_estado) VALUES ('completado'), ('cancelado') ON CONFLICT DO NOTHING;


-- ======================================================
-- INSERCIÓN DE DATOS REALISTAS PARA SISTEMA DE RESTAURANTES
-- Costa Rica: San José, Cartago, Heredia
-- ======================================================

DO $$
DECLARE
    -- Usuarios administradores
    admin_sj UUID;
    admin_cartago UUID;
    admin_heredia UUID;
    
    -- Cliente ejemplo
    cliente_ejemplo UUID;
    
    -- Restaurantes
    rest_sj UUID;
    rest_cartago UUID;
    rest_heredia UUID;
    
    -- Platos (IDs se generarán automáticamente)
BEGIN
    -- ======================================================
    -- 1. Crear usuarios (administradores y cliente)
    -- ======================================================
    
    -- Administrador del restaurante en San José
    INSERT INTO usuarios (id, nombre, email, keycloak_id, id_rol)
    VALUES (uuid_generate_v4(), 'Carlos Vargas Rodríguez', 'carlos.vargas@lafondacr.com', 'kc_admin_sj_001', 1)
    RETURNING id INTO admin_sj;
    
    -- Administrador del restaurante en Cartago
    INSERT INTO usuarios (id, nombre, email, keycloak_id, id_rol)
    VALUES (uuid_generate_v4(), 'Mariana Castro Jiménez', 'mariana.castro@antojosticos.cr', 'kc_admin_cartago_002', 1)
    RETURNING id INTO admin_cartago;
    
    -- Administrador del restaurante en Heredia
    INSERT INTO usuarios (id, nombre, email, keycloak_id, id_rol)
    VALUES (uuid_generate_v4(), 'Randall Fallas Solano', 'randall.fallas@umamihn.com', 'kc_admin_heredia_003', 1)
    RETURNING id INTO admin_heredia;
    
    -- Cliente de ejemplo
    INSERT INTO usuarios (id, nombre, email, keycloak_id, id_rol)
    VALUES (uuid_generate_v4(), 'Laura Fernández Segura', 'laura.fernandez@email.com', 'kc_cliente_001', 2)
    RETURNING id INTO cliente_ejemplo;
    
    -- ======================================================
    -- 2. Crear restaurantes
    -- ======================================================
    
    -- Restaurante 1: La Fonda Criolla (Comida típica costarricense) - San José
    INSERT INTO restaurantes (id, nombre, direccion, ciudad, telefono, id_administrador, capacidad_total)
    VALUES (uuid_generate_v4(), 'La Fonda Criolla', 'Avenida Central, entre calles 2 y 4, Frente al Parque Central, San José', 'San José', '+506 2256-7890', admin_sj, 80)
    RETURNING id INTO rest_sj;
    
    -- Restaurante 2: Antojos Ticos (Comida típica con fusión) - Cartago
    INSERT INTO restaurantes (id, nombre, direccion, ciudad, telefono, id_administrador, capacidad_total)
    VALUES (uuid_generate_v4(), 'Antojos Ticos Fusión', 'Barrio Los Ángeles, 200 metros norte de la Basílica, Cartago', 'Cartago', '+506 2551-3245', admin_cartago, 60)
    RETURNING id INTO rest_cartago;
    
    -- Restaurante 3: Umami House (Comida asiática) - Heredia
    INSERT INTO restaurantes (id, nombre, direccion, ciudad, telefono, id_administrador, capacidad_total)
    VALUES (uuid_generate_v4(), 'Umami House', 'San Joaquín de Flores, costado oeste del Mall Heredia, Heredia', 'Heredia', '+506 2262-9876', admin_heredia, 70)
    RETURNING id INTO rest_heredia;
    
    -- ======================================================
    -- 3. Insertar platos para cada restaurante
    -- ======================================================
    
    ----- RESTAURANTE: La Fonda Criolla (San José) - Comida típica costarricense -----
    INSERT INTO platos (id_restaurante, nombre, descripcion, precio, categoria)
    VALUES 
        (rest_sj, 'Chifrijo Gourmet', 'Tradicional chifrijo con chicharrón crujiente, frijoles negros, pico de gallo fresco, aguacate y tortillitas artesanales', 6500.00, 'tipica'),
        (rest_sj, 'Olla de Carne Montañesa', 'Sopa espesa con carne de res, yuca, plátano verde, papa, zanahoria, elote y culantro de monte, servida con arroz', 7800.00, 'tipica'),
        (rest_sj, 'Casado de Pescado al Guiso', 'Filete de pescado fresco guisado con salsa criolla, acompañado de arroz con palmito, frijoles molidos, ensalada y plátano maduro glaseado', 8900.00, 'tipica'),
        (rest_sj, 'Tortilla con Queso Turrialba', 'Tortilla de maíz recién hecha, rellena de queso Turrialba derretido, chicharrón y frijolitos molidos, con chimichurri casero', 5200.00, 'tipica'),
        (rest_sj, 'Ceviche de Corvina al Coco', 'Corvina fresca marinada en leche de coco, jugo de limón mandarina, jengibre, culantro y chile dulce, servido con camote crujiente', 8500.00, 'mariscos');
    
    ----- RESTAURANTE: Antojos Ticos Fusión (Cartago) - Fusión típica / Italiana -----
    INSERT INTO platos (id_restaurante, nombre, descripcion, precio, categoria)
    VALUES 
        (rest_cartago, 'Gallo Pinto Risotto', 'Risotto cremoso de arroz y frijoles negros con tocino caramelizado, huevo pochado y natilla reducida', 7800.00, 'fusion'),
        (rest_cartago, 'Pasta al Chipotle y Queso Palmito', 'Pasta artesanal con salsa cremosa de chipotle ahumado, queso palmito rallado y trocitos de pechuga de pollo', 8900.00, 'italiana'),
        (rest_cartago, 'Pizza Campesina Tica', 'Masa madre, salsa de tomate artesanal, queso Turrialba, chicharrón crujiente, frijoles refritos, elote asado y cilantrillo fresco', 11000.00, 'italiana'),
        (rest_cartago, 'Ñoquis de Ayote con Salsa de Frutos Rojos', 'Ñoquis suaves de ayote asado en salsa de zarzamoras y jengibre, cubiertos con queso caprino y nueces caramelizadas', 9500.00, 'italiana'),
        (rest_cartago, 'Tamal Asado con Relleno de Espinaca y Queso', 'Tamal horneado de masa de maíz tierno relleno de espinaca salteada y queso Turrialba, acompañado de crema ácida y salsa macha', 7200.00, 'tipica');
    
    ----- RESTAURANTE: Umami House (Heredia) - Comida asiática -----
    INSERT INTO platos (id_restaurante, nombre, descripcion, precio, categoria)
    VALUES 
        (rest_heredia, 'Ramen Tico-Ramen', 'Caldo Tonkotsu con toque de jengibre criollo, cerdo chashu, huevo marinado, maíz asado, cebolla morada encurtida y nori', 9500.00, 'asiatica'),
        (rest_heredia, 'Sushi de Pescado Blanco y Mango', 'Rollos de sushi con pescado blanco fresco, mango costarricense, aguacate, pepino y salsa teriyaki de coco', 12500.00, 'asiatica'),
        (rest_heredia, 'Curry Thai con Camarones', 'Curry rojo thai con leche de coco, camarones del Pacífico, piña asada, vainilla y hierbabuena, servido con arroz jazmín', 10800.00, 'asiatica'),
        (rest_heredia, 'Baos de Pollo Teriyaki', 'Panecillos al vapor chinos rellenos de pollo teriyaki glaseado, pepino encurtido, cebolla crujiente y salsa de cacahuate', 6900.00, 'asiatica'),
        (rest_heredia, 'Arroz Frito con Piña y Camarón', 'Arroz fuego wok con camarón tigre, trozos de piña caramelizada, huevo, cebollín y un toque de salsa de tamarindo', 8800.00, 'asiatica');
    
    -- ======================================================
    -- 4. Insertar una reserva de ejemplo (opcional)
    -- ======================================================
    INSERT INTO reservas (id_usuario, id_restaurante, fecha_reserva, cantidad_personas, id_estado)
    VALUES (
        cliente_ejemplo, 
        rest_sj, 
        CURRENT_TIMESTAMP + INTERVAL '3 days' + INTERVAL '20 hours', 
        4, 
        (SELECT id FROM estados_reserva WHERE nombre_estado = 'confirmada')
    );
    
    -- ======================================================
    -- 5. Insertar un pedido de ejemplo (para el cliente)
    -- ======================================================
    DECLARE
        pedido_id UUID;
        plato_ceviche UUID;
        plato_chifrijo UUID;
    BEGIN
        -- Obtener IDs de platos de La Fonda Criolla
        SELECT id INTO plato_ceviche FROM platos WHERE nombre = 'Ceviche de Corvina al Coco' AND id_restaurante = rest_sj;
        SELECT id INTO plato_chifrijo FROM platos WHERE nombre = 'Chifrijo Gourmet' AND id_restaurante = rest_sj;
        
        -- Crear pedido pendiente
        INSERT INTO pedidos (id, id_usuario, id_restaurante, es_recogida, id_estado_pedido, total)
        VALUES (uuid_generate_v4(), cliente_ejemplo, rest_sj, FALSE, 
                (SELECT id FROM estados_pedido WHERE nombre_estado = 'pendiente'), 0)
        RETURNING id INTO pedido_id;
        
        -- Insertar detalles del pedido
        INSERT INTO detalles_pedido (id_pedido, id_plato, cantidad, precio)
        VALUES 
            (pedido_id, plato_ceviche, 1, 8500.00),
            (pedido_id, plato_chifrijo, 2, 6500.00);
        
        -- Actualizar el total del pedido
        UPDATE pedidos 
        SET total = (8500.00 + (2 * 6500.00))
        WHERE id = pedido_id;
    END;
    
    -- Mensaje de éxito (opcional)
    RAISE NOTICE 'Datos insertados correctamente: 3 restaurantes, 15 platos, 4 usuarios, 1 reserva y 1 pedido de ejemplo';
    
END $$;