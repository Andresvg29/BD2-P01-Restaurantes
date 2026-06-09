CREATE VIEW IF NOT EXISTS v_horarios_pico AS
SELECT
    d.weekday,
    d.is_weekend,
    fo.hora,
    COUNT(DISTINCT fo.order_id) AS total_pedidos,
    ROUND(SUM(fo.total_amount), 2) AS ingresos_totales,
    COUNT(DISTINCT fo.user_key) AS usuarios_unicos,
    RANK() OVER (PARTITION BY d.weekday ORDER BY COUNT(DISTINCT fo.order_id) DESC) AS ranking_hora
FROM fact_orders fo
JOIN dim_date d ON fo.date_key = d.date_key
GROUP BY d.weekday, d.is_weekend, fo.hora;
