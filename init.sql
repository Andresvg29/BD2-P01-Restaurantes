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
INSERT INTO estados_pedido (nombre_estado) VALUES ('pendiente'), ('en_preparacion'), ('completado'), ('entregado') ON CONFLICT DO NOTHING;