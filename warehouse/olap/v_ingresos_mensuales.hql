CREATE VIEW IF NOT EXISTS v_ingresos_mensuales AS
SELECT
    d.year,
    d.month,
    r.restaurant_key,
    r.nombre AS restaurant_nombre,
    COUNT(DISTINCT fo.order_id) AS total_pedidos,
    ROUND(SUM(fo.total_amount), 2) AS ingresos_totales,
    ROUND(AVG(fo.total_amount), 2) AS ticket_promedio
FROM fact_orders fo
JOIN dim_date d ON fo.date_key = d.date_key
JOIN dim_restaurant r ON fo.restaurant_key = r.restaurant_key
GROUP BY d.year, d.month, r.restaurant_key, r.nombre;
