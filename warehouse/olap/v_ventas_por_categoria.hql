CREATE VIEW IF NOT EXISTS v_ventas_por_categoria AS
SELECT
    dp.categoria,
    COUNT(DISTINCT fod.detail_id) AS total_unidades_vendidas,
    SUM(fod.quantity) AS total_cantidad,
    ROUND(SUM(fod.subtotal), 2) AS ingresos_totales,
    ROUND(AVG(fod.unit_price), 2) AS precio_promedio,
    RANK() OVER (ORDER BY SUM(fod.subtotal) DESC) AS ranking_ingresos
FROM fact_order_details fod
JOIN dim_product dp ON fod.product_key = dp.product_key
GROUP BY dp.categoria;
