CREATE VIEW IF NOT EXISTS v_productividad_usuarios AS
SELECT
    du.user_key,
    du.user_id,
    du.nombre AS usuario_nombre,
    du.email,
    COUNT(DISTINCT fo.order_id) AS total_pedidos,
    ROUND(SUM(fo.total_amount), 2) AS gasto_total,
    ROUND(AVG(fo.total_amount), 2) AS ticket_promedio,
    COUNT(DISTINCT fo.restaurant_key) AS restaurantes_visitados,
    NTILE(5) OVER (ORDER BY SUM(fo.total_amount) DESC) AS percentil_gasto,
    CASE
        WHEN NTILE(5) OVER (ORDER BY SUM(fo.total_amount) DESC) <= 1 THEN 'VIP'
        WHEN NTILE(5) OVER (ORDER BY SUM(fo.total_amount) DESC) <= 3 THEN 'Frecuente'
        ELSE 'Ocasional'
    END AS segmento
FROM fact_orders fo
JOIN dim_user du ON fo.user_key = du.user_key
GROUP BY du.user_key, du.user_id, du.nombre, du.email;
